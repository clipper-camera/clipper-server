package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/clipper-camera/clipper-server/internal/helpers"
)

type UploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// RateLimitedReader wraps an io.Reader and limits the read rate
type RateLimitedReader struct {
	reader    io.Reader
	rate      int64 // bytes per second
	lastRead  time.Time
	bytesRead int64
}

func NewRateLimitedReader(reader io.Reader, rate int64) *RateLimitedReader {
	return &RateLimitedReader{
		reader:   reader,
		rate:     rate,
		lastRead: time.Now(),
	}
}

func (r *RateLimitedReader) Read(p []byte) (int, error) {
	// Calculate how many bytes we can read based on elapsed time
	elapsed := time.Since(r.lastRead).Seconds()
	allowedBytes := int64(elapsed * float64(r.rate))

	if allowedBytes <= 0 {
		// If we've read too much, wait until we can read more
		time.Sleep(time.Second / 10) // Sleep in small increments
		return 0, nil
	}

	// Limit the read size to the allowed bytes
	if int64(len(p)) > allowedBytes {
		p = p[:allowedBytes]
	}

	n, err := r.reader.Read(p)
	if n > 0 {
		r.bytesRead += int64(n)
		r.lastRead = time.Now()
	}
	return n, err
}

func (h *Handler) UploadMedia(w http.ResponseWriter, r *http.Request) {
	// Create a rate-limited reader for the request body (500KB/s = 500 * 1024 bytes/s)
	//rateLimitedBody := NewRateLimitedReader(r.Body, 500*1024)
	//r.Body = io.NopCloser(rateLimitedBody)

	// Load users from contacts file
	users, err := helpers.LoadUsers(h.cfg.ContactsFile)
	if err != nil {
		h.logger.Printf("Error loading users: %v\n", err)
		http.Error(w, "Unable to read contacts", http.StatusInternalServerError)
		return
	}

	// Parse multipart form without memory limit
	err = r.ParseMultipartForm(0) // 0 means no memory limit
	if err != nil {
		h.logger.Printf("Error parsing form: %v\n", err)
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Get media file
	file, header, err := r.FormFile("media")
	if err != nil {
		h.logger.Printf("Error getting media file: %v\n", err)
		http.Error(w, "Unable to get media file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get form values
	timestamp := r.FormValue("timestamp")
	recipientsJSON := r.FormValue("recipients")
	userPass := r.FormValue("userPass")
	mediaType := r.FormValue("mediaType")
	textOverlaysJSON := r.FormValue("textOverlays")

	// Find the authenticated user and validate password
	var currentUser *helpers.User
	for i, user := range users {
		if user.Password == userPass {
			currentUser = &users[i]
			break
		}
	}
	if currentUser == nil {
		h.logger.Printf("Invalid user password provided\n")
		http.Error(w, "Unauthorized user", http.StatusForbidden)
		return
	}

	// Create a map of friend IDs for quick lookup
	friendMap := make(map[int]bool)
	for _, friendID := range currentUser.Friends {
		friendMap[friendID] = true
	}

	// Parse recipients JSON
	var recipients []int
	if err := json.Unmarshal([]byte(recipientsJSON), &recipients); err != nil {
		h.logger.Printf("Error parsing recipients JSON: %v\n", err)
		http.Error(w, "Invalid recipients format", http.StatusBadRequest)
		return
	}

	// Filter recipients to only include friends
	var validRecipients []int
	for _, recipient := range recipients {
		if friendMap[recipient] {
			validRecipients = append(validRecipients, recipient)
		} else {
			h.logger.Printf("User attempted to send to non-friend user ID %d\n", recipient)
		}
	}

	if len(validRecipients) == 0 {
		h.logger.Printf("No valid recipients found\n")
		http.Error(w, "No valid recipients", http.StatusBadRequest)
		return
	}

	// Parse the timestamp as an int64 unix timestamp
	timestampInt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		h.logger.Printf("Error parsing timestamp: %v\n", err)
		http.Error(w, "Invalid timestamp format", http.StatusBadRequest)
		return
	}

	// Generate server-side timestamp for filename only
	serverTimestamp := fmt.Sprintf("%d", time.Now().UnixNano())

	// Create metadata exactly matching the form data
	metadata := map[string]interface{}{
		"timestamp":  timestampInt,
		"recipients": validRecipients,
		"userId":     currentUser.ID,
		"mediaType":  mediaType,
	}

	// Parse text overlays if provided
	if textOverlaysJSON != "" {
		var textOverlays []interface{}
		if err := json.Unmarshal([]byte(textOverlaysJSON), &textOverlays); err != nil {
			h.logger.Printf("Error parsing text overlays JSON: %v\n", err)
			http.Error(w, "Invalid text overlays format", http.StatusBadRequest)
			return
		}
		metadata["textOverlays"] = textOverlays
	}

	// Read the file content once
	fileContent, err := io.ReadAll(file)
	if err != nil {
		h.logger.Printf("Error reading file content: %v\n", err)
		http.Error(w, "Unable to read file content", http.StatusInternalServerError)
		return
	}

	// For each recipient, create their mailbox and save the file
	for _, recipient := range validRecipients {
		// Create recipient's mailbox directory
		mailboxDir := filepath.Join(h.cfg.MediaDir, "mailboxes", strconv.Itoa(recipient))
		if err := os.MkdirAll(mailboxDir, 0755); err != nil {
			h.logger.Printf("Error creating mailbox directory for %d: %v\n", recipient, err)
			continue
		}

		// Create filename with server-side timestamp
		fileExt := filepath.Ext(header.Filename)
		newFilename := fmt.Sprintf("%s%s", serverTimestamp, fileExt)
		filePath := filepath.Join(mailboxDir, newFilename)

		// Save the file
		if err := os.WriteFile(filePath, fileContent, 0644); err != nil {
			h.logger.Printf("Error saving file for %d: %v\n", recipient, err)
			continue
		}

		// Save metadata
		metadataPath := filePath + ".json"
		metadataFile, err := os.Create(metadataPath)
		if err != nil {
			h.logger.Printf("Error creating metadata file for %d: %v\n", recipient, err)
			continue
		}

		if err := json.NewEncoder(metadataFile).Encode(metadata); err != nil {
			h.logger.Printf("Error encoding metadata for %d: %v\n", recipient, err)
			metadataFile.Close()
			continue
		}
		metadataFile.Close()
	}

	// Send success response
	w.Header().Set("Content-Type", "application/json")
	response := UploadResponse{
		Success: true,
		Message: "File uploaded successfully",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Printf("Error encoding response: %v\n", err)
		http.Error(w, "Unable to send response", http.StatusInternalServerError)
		return
	}
}
