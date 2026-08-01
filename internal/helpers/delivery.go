package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MessageID strips the extension from a mailbox filename to get the id the
// sender was handed at upload time. Media and text messages share the naming
// scheme, so one message fanned out to several recipients keeps one id.
func MessageID(filename string) string {
	name := strings.TrimSuffix(filepath.Base(filename), ".json")
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// MarkDelivered records the first delivery of a mailbox item: it stamps
// firstDownloadedAt on the metadata, which is what the cleanup service keys off
// to expire the file, and drops a receipt for the sender to collect. Repeat
// calls are no-ops so a client re-reading its mailbox does not restart the
// expiry clock or duplicate the receipt.
func MarkDelivered(mediaDir, metadataPath string, metadata map[string]interface{}) error {
	if _, delivered := metadata["firstDownloadedAt"]; delivered {
		return nil
	}

	metadata["firstDownloadedAt"] = time.Now().UnixMilli()
	updated, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	if err := os.WriteFile(metadataPath, updated, 0644); err != nil {
		return err
	}

	return writeReceipt(mediaDir, metadataPath, metadata)
}

// writeReceipt leaves an empty file named "<messageID>.<recipientID>" in the
// sender's receipt directory. The name carries the whole payload, so delivery
// costs no storage beyond a directory entry, and the recipient is read back
// from the mailbox directory the item lives in.
func writeReceipt(mediaDir, metadataPath string, metadata map[string]interface{}) error {
	var senderID int
	switch v := metadata["userId"].(type) {
	case float64: // metadata read back from disk: JSON numbers decode as float64
		senderID = int(v)
	case int:
		senderID = v
	default:
		return fmt.Errorf("metadata %s has no usable userId", metadataPath)
	}

	recipientID := filepath.Base(filepath.Dir(metadataPath))
	receiptDir := filepath.Join(mediaDir, "receipts", fmt.Sprintf("%d", senderID))
	if err := os.MkdirAll(receiptDir, 0755); err != nil {
		return err
	}

	name := fmt.Sprintf("%s.%s", MessageID(metadataPath), recipientID)
	return os.WriteFile(filepath.Join(receiptDir, name), nil, 0644)
}
