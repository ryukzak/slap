package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/analytics"
	bcrypto "golang.org/x/crypto/bcrypt"

	passwordvalidator "github.com/wagslane/go-password-validator"
)

type resetPageData struct {
	Error    string
	UserID   string
	Username string
	Success  bool
}

// ResetRequestHandler lets an unauthenticated student generate a reset link.
// GET: show form asking for user ID. POST: generate token and show link.
func ResetRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		renderPage(w, "templates/reset_request_form.html", struct{ Error string }{})
		return
	}

	if err := r.ParseForm(); err != nil {
		renderPage(w, "templates/reset_request_form.html", struct{ Error string }{"Failed to parse form"})
		return
	}

	userID := strings.TrimSpace(r.FormValue("id"))
	if userID == "" {
		renderPage(w, "templates/reset_request_form.html", struct{ Error string }{"User ID is required"})
		return
	}

	if _, err := DB.GetUser(userID); err != nil {
		renderPage(w, "templates/reset_request_form.html", struct{ Error string }{"User not found"})
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Printf("Error generating reset token: %v", err)
		renderPage(w, "templates/reset_request_form.html", struct{ Error string }{"Failed to generate reset token"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	exp := time.Now().Add(24 * time.Hour)
	if err := DB.SetResetToken(userID, token, exp); err != nil {
		log.Printf("Error storing reset token for user %s: %v", userID, err)
		renderPage(w, "templates/reset_request_form.html", struct{ Error string }{"Failed to save reset token"})
		return
	}

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	resetLink := fmt.Sprintf("%s://%s/user/%s/reset?token=%s", scheme, r.Host, userID, token)

	log.Printf("action=reset_request user=%s", userID)
	renderPage(w, "templates/reset_request.html", struct{ ResetLink string }{resetLink})
}

// TeacherResetPasswordHandler lets a teacher set a new password for a student.
// GET: show form. POST: apply new password.
// Supports an optional ?token= query param for student-initiated reset links.
func TeacherResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	teacher := teacherSession(w, r)
	if teacher == nil {
		return
	}

	userID := mux.Vars(r)["userID"]
	dbUser, err := DB.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	token := r.URL.Query().Get("token")
	if token != "" {
		if dbUser.ResetToken == "" || dbUser.ResetToken != token || time.Now().After(dbUser.ResetTokenExp) {
			renderPage(w, "templates/reset.html", resetPageData{
				Error:    "Invalid or expired reset link",
				UserID:   userID,
				Username: dbUser.Username,
			})
			return
		}
	}

	if r.Method == http.MethodGet {
		renderPage(w, "templates/reset.html", resetPageData{UserID: userID, Username: dbUser.Username})
		return
	}

	if err := r.ParseForm(); err != nil {
		renderPage(w, "templates/reset.html", resetPageData{Error: "Failed to parse form", UserID: userID, Username: dbUser.Username})
		return
	}

	password := r.FormValue("password")
	if err := passwordvalidator.Validate(password, minEntropyBits); err != nil {
		renderPage(w, "templates/reset.html", resetPageData{
			Error:    fmt.Sprintf("Password error: %v", err),
			UserID:   userID,
			Username: dbUser.Username,
		})
		return
	}

	passwordHash, err := bcrypto.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		renderPage(w, "templates/reset.html", resetPageData{Error: "Error hashing password", UserID: userID, Username: dbUser.Username})
		return
	}

	if err := DB.UpdatePassword(userID, passwordHash); err != nil {
		log.Printf("Error updating password for user %s: %v", userID, err)
		renderPage(w, "templates/reset.html", resetPageData{Error: "Failed to update password", UserID: userID, Username: dbUser.Username})
		return
	}

	log.Printf("action=teacher_reset_password teacher=%s user=%s", teacher.ID, userID)
	renderPage(w, "templates/reset.html", resetPageData{Success: true, UserID: userID, Username: dbUser.Username})
}

// SettingsHandler shows the settings page for a user.
func SettingsHandler(w http.ResponseWriter, r *http.Request) {
	sessionUser := userSession(w, r)
	if sessionUser == nil {
		return
	}

	userID := mux.Vars(r)["userID"]
	if userID != sessionUser.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	dbUser, err := DB.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	renderPage(w, "templates/settings.html", settingsPageData{
		UserID:   userID,
		Username: dbUser.Username,
	})
}

type settingsPageData struct {
	UserID   string
	Username string
	Error    string
	Success  string
}

// SettingsPasswordHandler changes the user's password.
func SettingsPasswordHandler(w http.ResponseWriter, r *http.Request) {
	sessionUser := userSession(w, r)
	if sessionUser == nil {
		return
	}

	userID := mux.Vars(r)["userID"]
	if userID != sessionUser.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	dbUser, err := DB.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		renderPage(w, "templates/settings.html", settingsPageData{UserID: userID, Username: dbUser.Username, Error: "Failed to parse form"})
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")

	if err := bcrypto.CompareHashAndPassword(dbUser.PasswordHash, []byte(currentPassword)); err != nil {
		renderPage(w, "templates/settings.html", settingsPageData{UserID: userID, Username: dbUser.Username, Error: "Current password is incorrect"})
		return
	}

	if err := passwordvalidator.Validate(newPassword, minEntropyBits); err != nil {
		renderPage(w, "templates/settings.html", settingsPageData{
			UserID:   userID,
			Username: dbUser.Username,
			Error:    fmt.Sprintf("Password error: %v", err),
		})
		return
	}

	passwordHash, err := bcrypto.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		renderPage(w, "templates/settings.html", settingsPageData{UserID: userID, Username: dbUser.Username, Error: "Error hashing password"})
		return
	}

	if err := DB.UpdatePassword(userID, passwordHash); err != nil {
		log.Printf("Error updating password for user %s: %v", userID, err)
		renderPage(w, "templates/settings.html", settingsPageData{UserID: userID, Username: dbUser.Username, Error: "Failed to update password"})
		return
	}

	log.Printf("action=change_password user=%s", userID)
	renderPage(w, "templates/settings.html", settingsPageData{UserID: userID, Username: dbUser.Username, Success: "Password updated"})
}

// SettingsUsernameHandler changes the user's display name.
func SettingsUsernameHandler(w http.ResponseWriter, r *http.Request) {
	sessionUser := userSession(w, r)
	if sessionUser == nil {
		return
	}

	userID := mux.Vars(r)["userID"]
	if userID != sessionUser.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	dbUser, err := DB.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		renderPage(w, "templates/settings.html", settingsPageData{UserID: userID, Username: dbUser.Username, Error: "Failed to parse form"})
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	if username == "" {
		renderPage(w, "templates/settings.html", settingsPageData{UserID: userID, Username: dbUser.Username, Error: "Name cannot be empty"})
		return
	}

	if err := DB.UpdateUsername(userID, username); err != nil {
		log.Printf("Error updating username for user %s: %v", userID, err)
		renderPage(w, "templates/settings.html", settingsPageData{UserID: userID, Username: dbUser.Username, Error: "Failed to update name"})
		return
	}

	// Refresh JWT with new username
	isTeacher := AppConfig.IsTeacher(userID)
	tokenString, err := JwtAuth.GenerateToken(username, userID, true, isTeacher)
	if err == nil {
		setAuthCookie(w, tokenString)
	}

	log.Printf("action=change_username user=%s new_name=%s", userID, username)
	analytics.Identify(userID, username)
	renderPage(w, "templates/settings.html", settingsPageData{UserID: userID, Username: username, Success: "Name updated"})
}
