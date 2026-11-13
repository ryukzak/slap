package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/ryukzak/slap/src/auth"
	"github.com/ryukzak/slap/src/config"
	"github.com/ryukzak/slap/src/storage"
)

// setupSessionTest points the session globals at a fresh DB holding users and
// restores them when the test ends.
func setupSessionTest(t *testing.T, users ...*storage.UserData) {
	t.Helper()
	prevConfig, prevJwt, prevDB := AppConfig, JwtAuth, DB
	db, err := storage.NewDB(filepath.Join(t.TempDir(), "slap.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		AppConfig, JwtAuth, DB = prevConfig, prevJwt, prevDB
	})
	for _, u := range users {
		if err := db.SaveUser(u); err != nil {
			t.Fatal(err)
		}
	}
	DB = db
	JwtAuth = auth.NewJWTConfig([]byte("test_secret"), time.Hour)
	AppConfig = &config.Config{TeacherIDs: []string{"100"}}
}

func TestOptionalUserSessionTeacherFromConfig(t *testing.T) {
	setupSessionTest(t,
		&storage.UserData{ID: "100", Username: "user", IsStudent: true},
		&storage.UserData{ID: "200", Username: "user", IsStudent: true},
	)

	tests := []struct {
		name         string
		id           string
		tokenTeacher bool
		want         bool
	}{
		{"removed from config", "200", true, false},
		{"added to config", "100", false, true},
		{"still in config", "100", true, true},
		{"never a teacher", "200", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := JwtAuth.GenerateToken("user", tt.id, true, tt.tokenTeacher, 0)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "/", nil)
			r.AddCookie(&http.Cookie{Name: "user_data", Value: token})
			claims := optionalUserSession(r)
			if claims == nil {
				t.Fatal("optionalUserSession() = nil")
			}
			if claims.IsTeacher != tt.want {
				t.Errorf("IsTeacher = %v, want %v", claims.IsTeacher, tt.want)
			}
		})
	}
}

func TestOptionalUserSessionTokenVersion(t *testing.T) {
	setupSessionTest(t, &storage.UserData{ID: "300", Username: "user", IsStudent: true, TokenVersion: 1})

	tests := []struct {
		name    string
		version int
		want    bool
	}{
		{"issued before the password change", 0, false},
		{"current", 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := JwtAuth.GenerateToken("user", "300", true, false, tt.version)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "/", nil)
			r.AddCookie(&http.Cookie{Name: "user_data", Value: token})
			if got := optionalUserSession(r) != nil; got != tt.want {
				t.Errorf("session valid = %v, want %v", got, tt.want)
			}
		})
	}
}
