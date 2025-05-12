package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/clipper-camera/clipper-server/internal/helpers"
	"github.com/go-chi/chi/v5"
)

// customResponseWriter wraps http.ResponseWriter to track if the response was written successfully
type customResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *customResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.written = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *customResponseWriter) Write(b []byte) (int, error) {
	w.written = true
	return w.ResponseWriter.Write(b)
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	// Get user ID from URL path
	userPass := chi.URLParam(r, "user_password")
	filename := chi.URLParam(r, "filename")
	if userPass == "" || filename == "" {
		http.Error(w, "User ID and filename are required", http.StatusBadRequest)
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

	// Construct the file path
	filePath := filepath.Join(h.cfg.MediaDir, "mailboxes", strconv.Itoa(userId), filename)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		h.logger.Printf("Error opening file: %v\n", err)
		http.Error(w, "Unable to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get file info for content type and size
	fileInfo, err := file.Stat()
	if err != nil {
		h.logger.Printf("Error getting file info: %v\n", err)
		http.Error(w, "Unable to get file info", http.StatusInternalServerError)
		return
	}

	// Set headers for file download
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))

	// Create custom response writer to track successful write
	customWriter := &customResponseWriter{ResponseWriter: w}

	// Stream the file to the client
	http.ServeContent(customWriter, r, filename, fileInfo.ModTime(), file)

	// Only proceed if the response was written successfully and status code is 200
	metadataPath := filePath + ".json"
	if customWriter.written && customWriter.statusCode == http.StatusOK {
		// Read the current metadata
		metadataContent, err := os.ReadFile(metadataPath)
		if err != nil {
			h.logger.Printf("Error reading metadata file: %v\n", err)
			return
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal(metadataContent, &metadata); err != nil {
			h.logger.Printf("Error parsing metadata: %v\n", err)
			return
		}

		// Check if this is the first download
		_, exists := metadata["firstDownloadedAt"]
		if !exists {
			// This is the first download, set the timestamp in milliseconds
			metadata["firstDownloadedAt"] = time.Now().UnixMilli()
			updatedMetadata, err := json.Marshal(metadata)
			if err != nil {
				h.logger.Printf("Error marshaling metadata: %v\n", err)
				return
			}

			if err := os.WriteFile(metadataPath, updatedMetadata, 0644); err != nil {
				h.logger.Printf("Error updating metadata: %v\n", err)
				return
			}
		}
	}
}
