package handlers

import (
	"net/http"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	if claim := optionalUserSession(r); claim != nil {
		http.Redirect(w, r, "/user/"+claim.ID, http.StatusSeeOther)
		return
	}
	renderPage(w, "templates/auth.html", struct{ Error string }{})
}
