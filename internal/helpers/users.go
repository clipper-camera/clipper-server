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

// LookupUser returns the user holding the given password, or nil if none does.
// A nil user with a nil error means "bad password", not a server failure.
func LookupUser(contactsFile, password string) (*User, error) {
	users, err := LoadUsers(contactsFile)
	if err != nil {
		return nil, err
	}

	for i := range users {
		if users[i].Password == password {
			return &users[i], nil
		}
	}

	return nil, nil
} 