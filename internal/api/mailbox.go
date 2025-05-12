package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/clipper-camera/clipper-server/internal/helpers"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) GetMailbox(w http.ResponseWriter, r *http.Request) {
	// Get user ID from URL path
	userPass := chi.URLParam(r, "user_password")
	if userPass == "" {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Load users from contacts file
	users, err := helpers.LoadUsers(h.cfg.ContactsFile)
	if err != nil {
		h.logger.Printf("Error loading users: %v\n", err)
		http.Error(w, "Unable to read contacts", http.StatusInternalServerError)
		return
	}

	// Lets lookup the user ID from the password
	userId := -1
	for _, user := range users {
		if user.Password == userPass {
			userId = user.ID
		}
	}
	if userId == -1 {
		http.Error(w, "User ID is required", http.StatusBadRequest)
		return
	}

	// Construct the mailbox path
	mailboxPath := filepath.Join(h.cfg.MediaDir, "mailboxes", strconv.Itoa(userId))

	// Check if mailbox exists
	if _, err := os.Stat(mailboxPath); os.IsNotExist(err) {
		// Return empty array if mailbox doesn't exist
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	// Read all files in the mailbox
	files, err := os.ReadDir(mailboxPath)
	if err != nil {
		h.logger.Printf("Error reading mailbox directory: %v\n", err)
		http.Error(w, "Unable to read mailbox", http.StatusInternalServerError)
		return
	}

	// Process files and metadata
	var items []map[string]interface{}
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Skip metadata files
		if strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		// Read metadata file
		metadataPath := filepath.Join(mailboxPath, file.Name()+".json")
		metadataContent, err := os.ReadFile(metadataPath)
		if err != nil {
			h.logger.Printf("Error reading metadata for %s: %v\n", file.Name(), err)
			continue
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataContent, &metadata); err != nil {
			h.logger.Printf("Error parsing metadata for %s: %v\n", file.Name(), err)
			continue
		}

		// Remove recipients field
		delete(metadata, "recipients")

		// Add fileUrl to metadata
		metadata["fileUrl"] = "/_api/v1/download/" + userPass + "/" + file.Name()

		// Add mediaType to metadata
		if mediaType, ok := metadata["mediaType"]; ok {
			metadata["mediaType"] = mediaType
		}
		items = append(items, metadata)
	}

	// Sort items by timestamp (newest first)
	sort.Slice(items, func(i, j int) bool {
		ts1, ok1 := items[i]["timestamp"].(string)
		ts2, ok2 := items[j]["timestamp"].(string)
		if !ok1 || !ok2 {
			return false
		}
		return ts1 > ts2
	})

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if len(items) > 0 {
		if err := json.NewEncoder(w).Encode(items); err != nil {
			h.logger.Printf("Error encoding response: %v\n", err)
			http.Error(w, "Unable to send response", http.StatusInternalServerError)
			return
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
	}
}
