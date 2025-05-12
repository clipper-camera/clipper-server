package config

import "os"

type Config struct {
	Port         string
	ContactsFile string
	MediaDir     string
}

func (c *Config) Load() error {
	contactsFile := os.Getenv("CLIPPER_CONTACTS_FILE")
	if contactsFile == "" {
		contactsFile = "contacts.json"
	}
	c.ContactsFile = contactsFile

	port := os.Getenv("CLIPPER_PORT")
	if port == "" {
		port = "8080"
	}
	c.Port = port

	mediaDir := os.Getenv("CLIPPER_MEDIA_DIR")
	if mediaDir == "" {
		mediaDir = "media"
	}
	c.MediaDir = mediaDir

	// Create media directory if it doesn't exist
	if err := os.MkdirAll(mediaDir, 0755); err != nil {
		return err
	}

	return nil
}
