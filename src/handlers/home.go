package handlers

import (
	"net/http"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("user_data"); err == nil {
		if claim, err := JwtAuth.ExtractUserInfoWithRoles(cookie.Value); err == nil {
			http.Redirect(w, r, "/user/"+claim.ID, http.StatusSeeOther)
			return
		}
	}
	renderPage(w, "templates/auth.html", struct{ Error string }{})
}
