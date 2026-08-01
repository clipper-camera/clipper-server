package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/clipper-camera/clipper-server/internal/helpers"
	"github.com/go-chi/chi/v5"
)

type Receipt struct {
	MessageID   string `json:"messageId"`
	RecipientID int    `json:"recipientId"`
	DeliveredAt int64  `json:"deliveredAt"`
}

// GetReceipts returns, and then drops, the delivery confirmations waiting for
// this user: one per recipient that has picked up a message they sent. Holding
// them until collected is what lets a sender learn about a delivery that
// happened while their app was closed, since the message itself is gone by then.
func (h *Handler) GetReceipts(w http.ResponseWriter, r *http.Request) {
	userPass := chi.URLParam(r, "user_password")
	if userPass == "" {
		http.Error(w, "User password is required", http.StatusBadRequest)
		return
	}

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

	receiptDir := filepath.Join(h.cfg.MediaDir, "receipts", strconv.Itoa(user.ID))
	entries, err := os.ReadDir(receiptDir)
	if err != nil && !os.IsNotExist(err) {
		h.logger.Printf("Error reading receipts directory: %v\n", err)
		http.Error(w, "Unable to read receipts", http.StatusInternalServerError)
		return
	}

	// Receipts are empty files named "<messageID>.<recipientID>"
	receipts := []Receipt{}
	collected := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		split := strings.LastIndex(entry.Name(), ".")
		if split < 0 {
			continue
		}
		recipientID, err := strconv.Atoi(entry.Name()[split+1:])
		if err != nil {
			h.logger.Printf("Skipping malformed receipt %s: %v\n", entry.Name(), err)
			continue
		}

		var deliveredAt int64
		if info, err := entry.Info(); err == nil {
			deliveredAt = info.ModTime().UnixMilli()
		}

		receipts = append(receipts, Receipt{
			MessageID:   entry.Name()[:split],
			RecipientID: recipientID,
			DeliveredAt: deliveredAt,
		})
		collected = append(collected, filepath.Join(receiptDir, entry.Name()))
	}

	body, err := json.Marshal(receipts)
	if err != nil {
		h.logger.Printf("Error encoding receipts: %v\n", err)
		http.Error(w, "Unable to send response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(body); err != nil {
		// Leave the receipts in place so the next poll can retry them
		h.logger.Printf("Error sending receipts: %v\n", err)
		return
	}

	// ponytail: delete on read, so a response lost in transit loses the
	// confirmation. Ack the ids on the next poll instead if that ever matters.
	for _, path := range collected {
		if err := os.Remove(path); err != nil {
			h.logger.Printf("Error deleting receipt %s: %v\n", path, err)
		}
	}
}
