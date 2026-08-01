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

	// Lets lookup the user from the password
	user, err := helpers.LookupUser(h.cfg.ContactsFile, userPass)
	if err != nil {
		h.logger.Printf("Error loading users: %v\n", err)
		http.Error(w, "Unable to read contacts", http.StatusInternalServerError)
		return
	}
	if user == nil {
		http.Error(w, "Invalid user password", http.StatusForbidden)
		return
	}

	// Construct the mailbox path
	mailboxPath := filepath.Join(h.cfg.MediaDir, "mailboxes", strconv.Itoa(user.ID))

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

		// Stable per-message id, shared with the receipt the sender collects
		metadata["id"] = helpers.MessageID(file.Name())

		if metadata["mediaType"] == "text" {
			// A chat message has no second download step, handing the body over
			// in this response is the delivery, so the receipt fires here.
			content, err := os.ReadFile(filepath.Join(mailboxPath, file.Name()))
			if err != nil {
				h.logger.Printf("Error reading text message %s: %v\n", file.Name(), err)
				continue
			}
			metadata["text"] = string(content)

			if err := helpers.MarkDelivered(h.cfg.MediaDir, metadataPath, metadata); err != nil {
				h.logger.Printf("Error marking %s delivered: %v\n", file.Name(), err)
			}
		} else {
			metadata["fileUrl"] = "/_api/v1/download/" + userPass + "/" + file.Name()
			// So the client can show how big a clip is before committing to
			// downloading it
			if info, err := file.Info(); err == nil {
				metadata["fileSize"] = info.Size()
			}
		}

		// Remove recipients field, after the metadata above is persisted so the
		// stored copy keeps its full fanout list
		delete(metadata, "recipients")

		items = append(items, metadata)
	}

	// Sort items by timestamp (newest first). Timestamps are stored as unix
	// numbers, so they come back out of JSON as float64, not string.
	sort.Slice(items, func(i, j int) bool {
		ts1, ok1 := items[i]["timestamp"].(float64)
		ts2, ok2 := items[j]["timestamp"].(float64)
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
