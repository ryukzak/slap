package handlers

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/ryukzak/slap/src/storage"
)

// PrompterHandler renders the prompter: a teacher's private window, kept off
// the shared screen, that the other slap windows fill over a BroadcastChannel
// with the teacher notes of the student currently in focus.
func PrompterHandler(w http.ResponseWriter, r *http.Request) {
	sessionUser := teacherSession(w, r)
	if sessionUser == nil {
		return
	}
	renderPage(w, "templates/prompter.html", struct {
		SessionUserID string
	}{
		SessionUserID: sessionUser.ID,
	})
}

// PrompterNotesPartialHandler renders a student's teacher notes, with a form
// to add one, for the prompter.
func PrompterNotesPartialHandler(w http.ResponseWriter, r *http.Request) {
	if teacherSession(w, r) == nil {
		return
	}

	dbUser, err := DB.GetUser(mux.Vars(r)["userID"])
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	t, err := BaseTemplates.Clone()
	if err != nil {
		http.Error(w, "Failed to clone template", http.StatusInternalServerError)
		log.Printf("Template clone error: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	if err := t.ExecuteTemplate(w, "prompter_notes.html", struct {
		ID        storage.UserID
		Username  string
		UserGroup string
		Notes     []storage.UserNote
	}{
		ID:        dbUser.ID,
		Username:  dbUser.Username,
		UserGroup: dbUser.UserGroup,
		Notes:     dbUser.Notes,
	}); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		log.Printf("Template execution error: %v", err)
	}
}
