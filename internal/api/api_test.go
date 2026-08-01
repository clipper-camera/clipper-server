package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clipper-camera/clipper-server/internal/config"
)

const (
	alicePass = "alice-pass"
	bobPass   = "bob-pass"
)

// newTestServer wires a real router over a throwaway media dir and a two-user
// contact list where alice (1) and bob (2) are friends.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	dir := t.TempDir()
	contactsFile := filepath.Join(dir, "contacts.json")
	contacts := `[
		{"id": 1, "display_name": "Alice", "password": "` + alicePass + `", "friends": [2]},
		{"id": 2, "display_name": "Bob", "password": "` + bobPass + `", "friends": [1]}
	]`
	if err := os.WriteFile(contactsFile, []byte(contacts), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Port:         "0",
		ContactsFile: contactsFile,
		MediaDir:     filepath.Join(dir, "media"),
	}

	srv := httptest.NewServer(NewServer(context.Background(), cfg, log.New(io.Discard, "", 0)).Handler)
	t.Cleanup(srv.Close)
	return srv
}

// send posts a message. An empty filename means a text-only chat message.
func send(t *testing.T, srv *httptest.Server, fields map[string]string, filename string, content []byte) *http.Response {
	t.Helper()

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := form.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if filename != "" {
		part, err := form.CreateFormFile("media", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}

	resp, err := srv.Client().Post(srv.URL+"/_api/v1/upload", form.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func getJSON(t *testing.T, srv *httptest.Server, path string, out interface{}) {
	t.Helper()

	resp, err := srv.Client().Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
}

func messageID(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send: status %d", resp.StatusCode)
	}
	var out UploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if !out.Success || out.MessageID == "" {
		t.Fatalf("send: unusable response %+v", out)
	}
	return out.MessageID
}

// A text message is delivered inline by the mailbox read, which is what
// confirms it to the sender, and that confirmation is handed over exactly once.
func TestTextMessageRoundTrip(t *testing.T) {
	srv := newTestServer(t)

	id := messageID(t, send(t, srv, map[string]string{
		"userPass":   alicePass,
		"recipients": "[2]",
		"timestamp":  "1700000000000",
		"text":       "hey bob",
	}, "", nil))

	var receipts []Receipt
	getJSON(t, srv, "/_api/v1/receipts/"+alicePass, &receipts)
	if len(receipts) != 0 {
		t.Fatalf("receipt raised before bob read anything: %+v", receipts)
	}

	var mailbox []map[string]interface{}
	getJSON(t, srv, "/_api/v1/mailbox/"+bobPass, &mailbox)
	if len(mailbox) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mailbox))
	}
	if got := mailbox[0]["text"]; got != "hey bob" {
		t.Errorf("text = %v, want %q", got, "hey bob")
	}
	if got := mailbox[0]["id"]; got != id {
		t.Errorf("id = %v, want %v", got, id)
	}
	if got := mailbox[0]["mediaType"]; got != "text" {
		t.Errorf("mediaType = %v, want text", got)
	}
	if _, ok := mailbox[0]["recipients"]; ok {
		t.Error("mailbox leaked the recipient list")
	}

	getJSON(t, srv, "/_api/v1/receipts/"+alicePass, &receipts)
	if len(receipts) != 1 {
		t.Fatalf("expected 1 receipt, got %+v", receipts)
	}
	if receipts[0].MessageID != id || receipts[0].RecipientID != 2 {
		t.Errorf("receipt = %+v, want message %s to recipient 2", receipts[0], id)
	}
	if receipts[0].DeliveredAt == 0 {
		t.Error("receipt has no delivery time")
	}

	// Collected once and then gone: the server keeps no history
	getJSON(t, srv, "/_api/v1/receipts/"+alicePass, &receipts)
	if len(receipts) != 0 {
		t.Errorf("receipt served twice: %+v", receipts)
	}
}

// Media still takes the download path, so its receipt waits for the download
// rather than firing when the mailbox is merely listed.
func TestMediaMessageRoundTrip(t *testing.T) {
	srv := newTestServer(t)

	id := messageID(t, send(t, srv, map[string]string{
		"userPass":   bobPass,
		"recipients": "[1]",
		"timestamp":  "1700000000000",
		"mediaType":  "image",
	}, "clip.jpg", []byte("jpeg-bytes")))

	var mailbox []map[string]interface{}
	getJSON(t, srv, "/_api/v1/mailbox/"+alicePass, &mailbox)
	if len(mailbox) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mailbox))
	}
	fileURL, ok := mailbox[0]["fileUrl"].(string)
	if !ok {
		t.Fatalf("media message has no fileUrl: %+v", mailbox[0])
	}

	var receipts []Receipt
	getJSON(t, srv, "/_api/v1/receipts/"+bobPass, &receipts)
	if len(receipts) != 0 {
		t.Fatalf("receipt raised before the download: %+v", receipts)
	}

	resp, err := srv.Client().Get(srv.URL + fileURL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if string(got) != "jpeg-bytes" {
		t.Errorf("downloaded %q, want %q", got, "jpeg-bytes")
	}

	getJSON(t, srv, "/_api/v1/receipts/"+bobPass, &receipts)
	if len(receipts) != 1 || receipts[0].MessageID != id || receipts[0].RecipientID != 1 {
		t.Fatalf("receipts = %+v, want message %s to recipient 1", receipts, id)
	}
}

func TestSendRejectsBadRequests(t *testing.T) {
	base := map[string]string{"userPass": alicePass, "recipients": "[2]", "timestamp": "1700000000000"}

	withField := func(key, value string) map[string]string {
		fields := map[string]string{}
		for k, v := range base {
			fields[k] = v
		}
		fields[key] = value
		return fields
	}

	tests := []struct {
		name   string
		fields map[string]string
		status int
	}{
		{"no media and no text", base, http.StatusBadRequest},
		{"text over the cap", withField("text", strings.Repeat("x", MaxTextLength+1)), http.StatusBadRequest},
		{"recipient is not a friend", map[string]string{
			"userPass": alicePass, "recipients": "[99]", "timestamp": "1700000000000", "text": "hi",
		}, http.StatusBadRequest},
		{"bad password", map[string]string{
			"userPass": "nope", "recipients": "[2]", "timestamp": "1700000000000", "text": "hi",
		}, http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newTestServer(t)
			resp := send(t, srv, tt.fields, "", nil)
			defer resp.Body.Close()

			if resp.StatusCode != tt.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.status)
			}

			var mailbox []map[string]interface{}
			getJSON(t, srv, "/_api/v1/mailbox/"+bobPass, &mailbox)
			if len(mailbox) != 0 {
				t.Errorf("rejected message was still delivered: %+v", mailbox)
			}
		})
	}
}
