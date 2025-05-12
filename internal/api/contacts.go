package api

import (
	"encoding/json"
	"net/http"

	"github.com/clipper-camera/clipper-server/internal/helpers"
	"github.com/go-chi/chi/v5"
)

type Contact struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
}

func (h *Handler) GetContacts(w http.ResponseWriter, r *http.Request) {
	// Get user password from URL path
	userPass := chi.URLParam(r, "user_password")
	if userPass == "" {
		http.Error(w, "User password is required", http.StatusBadRequest)
		return
	}

	// Load users from contacts file
	users, err := helpers.LoadUsers(h.cfg.ContactsFile)
	if err != nil {
		h.logger.Printf("Error loading users: %v\n", err)
		http.Error(w, "Unable to read contacts", http.StatusInternalServerError)
		return
	}

	// Find the authenticated user and validate password
	var currentUser *helpers.User
	for i, user := range users {
		if user.Password == userPass {
			currentUser = &users[i]
			break
		}
	}
	if currentUser == nil {
		http.Error(w, "Invalid user password", http.StatusForbidden)
		return
	}

	// Create a map of friend IDs for quick lookup
	friendMap := make(map[int]bool)
	for _, friendID := range currentUser.Friends {
		friendMap[friendID] = true
	}

	// Filter users to only include friends
	var contacts []Contact
	for _, user := range users {
		if friendMap[user.ID] {
			contacts = append(contacts, Contact{
				ID:          user.ID,
				DisplayName: user.DisplayName,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(contacts)
	if err != nil {
		h.logger.Printf("Error encoding contacts: %v\n", err)
		http.Error(w, "Unable to encode contacts", http.StatusInternalServerError)
		return
	}
}
