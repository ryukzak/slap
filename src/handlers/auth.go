package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ryukzak/slap/src/analytics"
	"github.com/ryukzak/slap/src/auth"
	"github.com/ryukzak/slap/src/storage"
	passwordvalidator "github.com/wagslane/go-password-validator"
	bcrypto "golang.org/x/crypto/bcrypt"
)

const minEntropyBits = 70
const bcryptCost = 12

// SecureCookies controls whether the auth cookie is set with Secure flag.
// Set to true in production (HTTPS). Configurable via SLAP_SECURE_COOKIES env var.
var SecureCookies bool

func setAuthCookie(w http.ResponseWriter, tokenString string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "user_data",
		Value:    tokenString,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
}

func userSession(w http.ResponseWriter, r *http.Request) *auth.UserClaims {
	cookie, err := r.Cookie("user_data")
	if err != nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return nil
	}

	userClaim, err := JwtAuth.ExtractUserInfoWithRoles(cookie.Value)
	if err != nil {
		http.Error(w, "Invalid token", http.StatusUnauthorized)
		return nil
	}

	return userClaim
}

func teacherSession(w http.ResponseWriter, r *http.Request) *auth.UserClaims {
	userClaim := userSession(w, r)
	if userClaim == nil || !userClaim.IsTeacher {
		http.Error(w, "Only teachers can do that", http.StatusForbidden)
		return nil
	}
	return userClaim
}

func studentSession(w http.ResponseWriter, r *http.Request) *auth.UserClaims {
	userClaim := userSession(w, r)
	if userClaim == nil || !userClaim.IsStudent {
		http.Error(w, "Only students can do that", http.StatusForbidden)
		return nil
	}
	return userClaim
}

func renderAuthPageWithError(w http.ResponseWriter, errorMsg string) {
	renderAuthPage(w, errorMsg, "")
}

func renderAuthPage(w http.ResponseWriter, errorMsg, resetUserID string) {
	data := struct {
		Error       string
		ResetUserID string
	}{
		Error:       errorMsg,
		ResetUserID: resetUserID,
	}
	renderPage(w, "templates/auth.html", data)
}

func SignupHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderAuthPageWithError(w, "Failed to parse form")
		return
	}

	id := r.FormValue("id")
	password := r.FormValue("password")
	username := r.FormValue("username")

	if id == "" || password == "" || username == "" {
		renderAuthPageWithError(w, "All fields are required")
		return
	}

	if err := passwordvalidator.Validate(password, minEntropyBits); err != nil {
		renderAuthPageWithError(w, fmt.Sprintf("Password error: %v", err))
		return
	}

	_, err := DB.GetUser(id)
	if err == nil {
		renderAuthPageWithError(w, "User already exists")
		return
	}

	passwordHash, err := bcrypto.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		renderAuthPageWithError(w, "Error generating password hash")
		return
	}

	// All signups create students; teacher status is determined by config
	isTeacher := AppConfig.IsTeacher(id)
	tokenString, err := JwtAuth.GenerateToken(username, id, true, isTeacher)
	if err != nil {
		renderAuthPageWithError(w, "Error generating token")
		return
	}

	userData := &storage.UserData{
		ID:           id,
		Username:     username,
		PasswordHash: passwordHash,
		IsStudent:    true,
		IsTeacher:    isTeacher,
		UserGroup:    "student",
	}

	if err := DB.SaveUser(userData); err != nil {
		renderAuthPageWithError(w, "Error creating user")
		log.Printf("Error saving user: %v", err)
		return
	}

	log.Printf("action=signup user=%s", id)
	analytics.Identify(id, username)
	analytics.Track(id, "sign_up", map[string]any{"is_teacher": isTeacher})
	setAuthCookie(w, tokenString)
	redirectTo(w, r, "/user/"+id)
}

func SigninHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		renderAuthPageWithError(w, "Failed to parse form")
		return
	}

	id := r.FormValue("id")
	password := r.FormValue("password")

	if id == "" || password == "" {
		renderAuthPageWithError(w, "ID and password are required")
		return
	}

	if signinLimiter.isLocked(id) {
		renderAuthPageWithError(w, "Too many failed attempts. Try again later.")
		return
	}

	user, err := DB.GetUser(id)
	if err != nil {
		signinLimiter.recordFailure(id)
		renderAuthPageWithError(w, "Invalid ID or password")
		return
	}

	if err := bcrypto.CompareHashAndPassword(user.PasswordHash, []byte(password)); err != nil {
		signinLimiter.recordFailure(id)
		renderAuthPage(w, "Invalid ID or password", id)
		return
	}

	signinLimiter.reset(id)

	// Teacher status is determined by config, not stored value
	isTeacher := AppConfig.IsTeacher(user.ID)
	tokenString, err := JwtAuth.GenerateToken(user.Username, user.ID, true, isTeacher)
	if err != nil {
		renderAuthPageWithError(w, "Error generating token")
		return
	}

	log.Printf("action=signin user=%s", user.ID)
	analytics.Identify(user.ID, user.Username)
	analytics.Track(user.ID, "sign_in", map[string]any{"is_teacher": isTeacher})
	setAuthCookie(w, tokenString)
	redirectTo(w, r, "/user/"+user.ID)
}

// redirectTo performs a full-page redirect. For HTMX requests it uses HX-Redirect
// so the browser URL changes; for plain requests it uses 303 See Other.
func redirectTo(w http.ResponseWriter, r *http.Request, url string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, url, http.StatusSeeOther)
	}
}

// TokenHandler shows the current user's JWT token (requires authentication).
// Useful for API access.
func TokenHandler(w http.ResponseWriter, r *http.Request) {
	claims := userSession(w, r)
	if claims == nil {
		return
	}

	cookie, err := r.Cookie("user_data")
	if err != nil {
		renderAuthPageWithError(w, "No active session")
		return
	}

	data := struct {
		Token     string
		Link      string
		Username  string
		ID        storage.UserID
		IsStudent bool
		IsTeacher bool
	}{
		Token:     cookie.Value,
		Link:      "/user/" + claims.ID,
		Username:  claims.Username,
		ID:        claims.ID,
		IsStudent: claims.IsStudent,
		IsTeacher: claims.IsTeacher,
	}

	renderPage(w, "templates/token.html", data)
}

func SetCookieHandler(w http.ResponseWriter, r *http.Request) {
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		renderAuthPageWithError(w, "Token is required")
		return
	}

	claims, err := JwtAuth.ValidateToken(tokenString)
	if err != nil {
		renderAuthPageWithError(w, "Invalid or expired token")
		return
	}

	setAuthCookie(w, tokenString)
	http.Redirect(w, r, "/user/"+claims.ID, http.StatusSeeOther)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "user_data",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func getUserClaim(w http.ResponseWriter, r *http.Request) *auth.UserClaims {
	cookie, err := r.Cookie("user_data")
	if err != nil {
		http.Error(w, "Authentication required, please login first", http.StatusUnauthorized)
		return nil
	}

	userClaim, err := JwtAuth.ExtractUserInfoWithRoles(cookie.Value)
	if err != nil {
		invalidateCookie(w)
		return nil
	}
	return userClaim
}

func invalidateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "user_data",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Secure:   SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})

	http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
}
