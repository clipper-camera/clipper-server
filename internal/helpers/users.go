package helpers

import (
	"encoding/json"
	"io"
	"os"
)

type User struct {
	ID          int    `json:"id"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
	Friends     []int  `json:"friends"`
}

// LoadUsers reads and parses the contacts file, returning the list of users
func LoadUsers(contactsFile string) ([]User, error) {
	file, err := os.Open(contactsFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var users []User
	if err := json.Unmarshal(byteValue, &users); err != nil {
		return nil, err
	}

	return users, nil
} 