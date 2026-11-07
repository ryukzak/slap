package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ryukzak/slap/src/auth"
	"github.com/ryukzak/slap/src/config"
)

func TestOptionalUserSessionTeacherFromConfig(t *testing.T) {
	prevConfig, prevJwt := AppConfig, JwtAuth
	t.Cleanup(func() { AppConfig, JwtAuth = prevConfig, prevJwt })

	JwtAuth = auth.NewJWTConfig([]byte("test_secret"), time.Hour)
	AppConfig = &config.Config{TeacherIDs: []string{"100"}}

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
			token, err := JwtAuth.GenerateToken("user", tt.id, true, tt.tokenTeacher)
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
