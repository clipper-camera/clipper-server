package helpers

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/clipper-camera/clipper-server/internal/config"
)

const (
	// FileRetentionDuration is how long files are kept after being downloaded
	FileRetentionDuration = 10 * time.Minute
	// CleanupCheckInterval is how often we check for files to delete
	CleanupCheckInterval = 2 * time.Minute
)

type CleanupService struct {
	cfg    *config.Config
	logger *log.Logger
}

func NewCleanupService(cfg *config.Config, logger *log.Logger) *CleanupService {
	return &CleanupService{
		cfg:    cfg,
		logger: logger,
	}
}

func (s *CleanupService) Start() {
	go s.cleanupExpiredFiles()
}

func (s *CleanupService) cleanupExpiredFiles() {
	for {
		// Get the mailboxes directory
		mailboxesDir := filepath.Join(s.cfg.MediaDir, "mailboxes")

		// Read all user directories
		userDirs, err := os.ReadDir(mailboxesDir)
		if err != nil {
			s.logger.Printf("Error reading mailboxes directory: %v\n", err)
			time.Sleep(2 * time.Minute)
			continue
		}

		// Process each user's mailbox
		for _, userDir := range userDirs {
			if !userDir.IsDir() {
				continue
			}

			userPath := filepath.Join(mailboxesDir, userDir.Name())
			files, err := os.ReadDir(userPath)
			if err != nil {
				s.logger.Printf("Error reading user directory %s: %v\n", userDir.Name(), err)
				continue
			}

			// Process each file in the user's mailbox
			for _, file := range files {
				if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
					continue
				}

				// Read the metadata file
				metadataPath := filepath.Join(userPath, file.Name())
				metadataContent, err := os.ReadFile(metadataPath)
				if err != nil {
					s.logger.Printf("Error reading metadata file %s: %v\n", file.Name(), err)
					continue
				}

				var metadata map[string]interface{}
				if err := json.Unmarshal(metadataContent, &metadata); err != nil {
					s.logger.Printf("Error parsing metadata file %s: %v\n", file.Name(), err)
					continue
				}

				// Check if the file has been downloaded and is ready for deletion
				if firstDownloadedAt, ok := metadata["firstDownloadedAt"].(float64); ok {
					// Convert milliseconds to time.Time
					downloadTime := time.UnixMilli(int64(firstDownloadedAt))
					timeUntilDeletion := FileRetentionDuration - time.Since(downloadTime)

					if time.Since(downloadTime) >= FileRetentionDuration {
						// Delete the media file (remove .json extension)
						mediaPath := strings.TrimSuffix(metadataPath, ".json")
						mediaFileName := filepath.Base(mediaPath)

						// Get file info for logging
						fileInfo, err := os.Stat(mediaPath)
						if err != nil {
							s.logger.Printf("Error getting file info for %s: %v\n", mediaPath, err)
							continue
						}

						// Format the deletion message
						s.logger.Printf("🗑️  Deleting expired file: %s (user: %s, size: %d bytes, type: %s, downloaded: %s, age: %s)\n",
							mediaFileName,
							userDir.Name(),
							fileInfo.Size(),
							metadata["mediaType"],
							downloadTime.Format("2006-01-02 15:04:05.000"),
							time.Since(downloadTime).Round(time.Second),
						)

						// Delete the files
						if err := os.Remove(mediaPath); err != nil {
							s.logger.Printf("Error deleting media file %s: %v\n", mediaPath, err)
						}

						if err := os.Remove(metadataPath); err != nil {
							s.logger.Printf("Error deleting metadata file %s: %v\n", metadataPath, err)
						}
					} else {
						// Log files that will be deleted in the future
						mediaFileName := strings.TrimSuffix(filepath.Base(metadataPath), ".json")
						s.logger.Printf("⏳ File pending deletion: %s (user: %s, type: %s, downloaded: %s, time until deletion: %s)\n",
							mediaFileName,
							userDir.Name(),
							metadata["mediaType"],
							downloadTime.Format("2006-01-02 15:04:05.000"),
							timeUntilDeletion.Round(time.Second),
						)
					}
				}
			}
		}

		// Sleep for the retention time before next cleanup
		time.Sleep(CleanupCheckInterval)
	}
}
