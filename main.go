package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/analytics"
	"github.com/ryukzak/slap/src/auth"
	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/handlers"
	"github.com/ryukzak/slap/src/storage"
	"github.com/ryukzak/slap/src/util"
)

var jwtAuth *auth.JWTConfig
var templates *template.Template
var db *storage.DB
var appConfig *config.Config

const defaultDBPath = "tmp/slap.db"

var port = flag.String("port", func() string {
	if p := os.Getenv("SLAP_PORT"); p != "" {
		return p
	}
	return "8080"
}(), "Port to run the server on (env: SLAP_PORT)")

var version = "dev"

const maxRequestBodyBytes = 64 * 1024 // 64 KB

func limitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self' https://unpkg.com https://cdn.tailwindcss.com; "+
				"style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: https:; "+
				"font-src 'self'; "+
				"connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func init() {
	flag.Parse()

	posthogKey := os.Getenv("SLAP_POSTHOG_KEY")
	if posthogKey == "" {
		posthogKey = "phc_4MVBHknwF8Qok57n2J5S9OVP3z6BpRJM4fiDtH7rGg7"
	}
	analytics.Init(posthogKey, os.Getenv("SLAP_POSTHOG_HOST"), version)

	primaryTZ := os.Getenv("SLAP_TZ")
	if primaryTZ == "" {
		primaryTZ = "Europe/Moscow"
	}
	primaryLoc, err := time.LoadLocation(primaryTZ)
	if err != nil {
		log.Fatalf("Invalid SLAP_TZ %q: %v", primaryTZ, err)
	}
	primaryTZName := time.Now().In(primaryLoc).Format("MST")

	secondaryLoc, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		log.Fatal(err)
	}

	handlers.PrimaryLoc = primaryLoc
	handlers.PrimaryTZName = primaryTZName
	handlers.StartTime = time.Now()

	funcMap := template.FuncMap{
		"markdown":       util.RenderMarkdown,
		"formatDateTime": util.FormatDateTime(primaryTZName, primaryLoc, "CET", secondaryLoc),
		"sub": func(a, b int) int {
			return a - b
		},
		"getTitle":    util.GetTitle,
		"getRestText": util.GetRestText,
		"boldScore": func(s string) template.HTML {
			return template.HTML(util.BoldScore(s)) //nolint:gosec
		},
		"appVersion": func() string { return version },
		"uptime": func() string {
			d := time.Since(handlers.StartTime)
			h := int(d.Hours())
			m := int(d.Minutes()) % 60
			return fmt.Sprintf("%dh%02dm", h, m)
		},
	}

	templates = template.Must(template.New("").Funcs(funcMap).ParseGlob("templates/partials/*.html"))
	handlers.BaseTemplates = templates

	jwtSecret := os.Getenv("SLAP_JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("SLAP_JWT_SECRET environment variable is required")
	}
	jwtAuth = auth.NewJWTConfig([]byte(jwtSecret), 24*time.Hour)

	handlers.SecureCookies = os.Getenv("SLAP_SECURE_COOKIES") == "true"

	dbPath := defaultDBPath
	if envPath := os.Getenv("SLAP_DB"); envPath != "" {
		dbPath = envPath
	}
	log.Printf("Database: %s", dbPath)
	db, err = storage.NewDB(dbPath, "")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	handlers.Templates = templates
	handlers.JwtAuth = jwtAuth
	handlers.DB = db
	handlers.Version = version
}

func main() {
	// Get default config path from environment variable or use default
	defaultConfigPath := "conf/config.yaml"
	if envPath := os.Getenv("SLAP_CONF"); envPath != "" {
		defaultConfigPath = envPath
	}

	configPath := flag.String("config", defaultConfigPath, "Path to the configuration file")
	flag.Parse()

	// Load configuration
	log.Printf("Config: %s", *configPath)
	var err error
	appConfig, err = config.LoadConfig(*configPath)
	if err != nil {
		log.Printf("Warning: Failed to load configuration file: %v", err)
		log.Println("Using default configuration")
		appConfig = config.DefaultConfig()
	}

	// Update handlers package AppConfig with the loaded config
	handlers.AppConfig = appConfig

	defer analytics.Close()
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}()

	r := mux.NewRouter()

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Routes
	r.HandleFunc("/", handlers.HomeHandler).Methods("GET")
	r.HandleFunc("/signup", handlers.SignupHandler).Methods("POST")
	r.HandleFunc("/signin", handlers.SigninHandler).Methods("POST")
	r.HandleFunc("/token", handlers.TokenHandler).Methods("GET")
	r.HandleFunc("/set-cookie", handlers.SetCookieHandler).Methods("GET")
	r.HandleFunc("/logout", handlers.LogoutHandler).Methods("GET")

	// Common
	r.HandleFunc("/parts/user-line", handlers.UserLineHandler).Methods("GET")

	// User and task routes with specific path prefixes
	r.HandleFunc("/users", handlers.UserListHandler).Methods("GET")
	r.HandleFunc("/user/{userID}", handlers.UserInfoHandler).Methods("GET")
	r.HandleFunc("/user/{userID}/task/{taskID}", handlers.TaskDetailHandler).Methods("GET")
	r.HandleFunc("/user/{userID}/task/{taskID}/journal", handlers.AddTaskRecordHandler).Methods("POST")
	r.HandleFunc("/user/{userID}/settings", handlers.SettingsHandler).Methods("GET")
	r.HandleFunc("/user/{userID}/settings/password", handlers.SettingsPasswordHandler).Methods("POST")
	r.HandleFunc("/user/{userID}/settings/username", handlers.SettingsUsernameHandler).Methods("POST")

	// Password reset (teacher-only)
	r.HandleFunc("/user/{userID}/reset", handlers.TeacherResetPasswordHandler).Methods("GET", "POST")

	// Lesson routes
	r.HandleFunc("/api/lessons", handlers.CreateLessonHandler).Methods("POST")
	r.HandleFunc("/lesson/{lessonID}", handlers.LessonDetailHandler).Methods("GET")
	r.HandleFunc("/lesson/{lessonID}/records", handlers.LessonTaskRecordsPartialHandler).Methods("GET")
	r.HandleFunc("/api/lessons", handlers.RenderLessonListHandler).Methods("GET")
	r.HandleFunc("/api/lesson/{lessonID}/register", handlers.RegisterTaskRecordToLessonHandler).Methods("POST")
	r.HandleFunc("/api/lesson/{lessonID}/unregister", handlers.UnregisterFromLessonHandler).Methods("POST")
	r.HandleFunc("/api/lessons/{lessonID}", handlers.DeleteLessonHandler).Methods("DELETE")

	// Start server
	fmt.Printf("Server started at http://localhost:%s (version: %s)\n", *port, version)
	log.Fatal(http.ListenAndServe(":"+*port, securityHeaders(limitRequestBody(r))))
}
