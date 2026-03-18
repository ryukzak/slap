package storage

import (
	"encoding/json"
	"fmt"
	"time"

	bolt "go.etcd.io/bbolt"
)

type UserID = string

type UserData struct {
	ID            UserID `json:"id"`
	PasswordHash  []byte
	Username      string                  `json:"username"`
	UserGroup     string                  `json:"user_group"`
	CreatedAt     time.Time               `json:"created_at"`
	IsStudent     bool                    `json:"is_student"`
	IsTeacher     bool                    `json:"is_teacher"`
	LessonIDs     []LessonID              `json:"lesson_ids"`
	Journals      map[TaskID][]TaskRecord `json:"journals"`
	ResetToken    string                  `json:"reset_token,omitempty"`
	ResetTokenExp time.Time               `json:"reset_token_exp,omitempty"`
}

func (d *DB) SaveUser(userData *UserData) error {
	if userData.ID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)

		// Check if user exists
		existingData := b.Get([]byte(userData.ID))
		if existingData != nil {
			// User exists, only update username if needed
			var existingUser UserData
			if err := json.Unmarshal(existingData, &existingUser); err != nil {
				return fmt.Errorf("could not unmarshal existing user data: %w", err)
			}

			// Update username if different
			changed := false
			if existingUser.Username != userData.Username {
				existingUser.Username = userData.Username
				changed = true
			}

			// Update role flags if different
			if existingUser.IsStudent != userData.IsStudent {
				existingUser.IsStudent = userData.IsStudent
				changed = true
			}
			if existingUser.IsTeacher != userData.IsTeacher {
				existingUser.IsTeacher = userData.IsTeacher
				changed = true
			}

			if existingUser.UserGroup != userData.UserGroup && userData.UserGroup != "" {
				existingUser.UserGroup = userData.UserGroup
				changed = true
			}

			if len(userData.PasswordHash) > 0 {
				existingUser.PasswordHash = userData.PasswordHash
				changed = true
			}

			// Preserve existing journals if not updating them
			if len(userData.Journals) > 0 {
				existingUser.Journals = userData.Journals
				changed = true
			}

			// Preserve existing lesson IDs if not updating them
			if len(userData.LessonIDs) > 0 {
				existingUser.LessonIDs = userData.LessonIDs
				changed = true
			}

			// If nothing changed, return early
			if !changed {
				return nil
			}

			buf, err := json.Marshal(existingUser)
			if err != nil {
				return fmt.Errorf("could not marshal user data: %w", err)
			}

			return b.Put([]byte(userData.ID), buf)
		}

		// New user, set creation time
		now := time.Now()
		if userData.CreatedAt.IsZero() {
			userData.CreatedAt = now
		}

		// Initialize journals if nil
		if userData.Journals == nil {
			userData.Journals = make(map[TaskID][]TaskRecord)
		}

		// Initialize lesson IDs if nil
		if userData.LessonIDs == nil {
			userData.LessonIDs = []LessonID{}
		}

		if err := appendToIndex(b, "users", userData.ID); err != nil {
			return err
		}

		buf, err := json.Marshal(userData)
		if err != nil {
			return fmt.Errorf("could not marshal user data: %w", err)
		}

		return b.Put([]byte(userData.ID), buf)
	})
}

func (d *DB) ListUsers() ([]*UserData, error) {
	var users []*UserData
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		ids, err := getIndex(b, "users")
		if err != nil {
			return err
		}
		for _, id := range ids {
			user, err := getValue[UserData](b, id)
			if err != nil {
				return err
			}
			users = append(users, user)
		}
		return nil
	})
	return users, err
}

func (d *DB) SetResetToken(userID, token string, exp time.Time) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		user, err := getValue[UserData](b, userID)
		if err != nil {
			return err
		}
		user.ResetToken = token
		user.ResetTokenExp = exp
		return setValue(b, userID, *user)
	})
}

func (d *DB) UpdatePassword(userID string, passwordHash []byte) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		user, err := getValue[UserData](b, userID)
		if err != nil {
			return err
		}
		user.PasswordHash = passwordHash
		user.ResetToken = ""
		user.ResetTokenExp = time.Time{}
		return setValue(b, userID, *user)
	})
}

func (d *DB) UpdateIsTeacher(userID UserID, isTeacher bool) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		user, err := getValue[UserData](b, userID)
		if err != nil {
			return err
		}
		user.IsTeacher = isTeacher
		return setValue(b, userID, *user)
	})
}

func (d *DB) UpdateUsername(userID, username string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		user, err := getValue[UserData](b, userID)
		if err != nil {
			return err
		}
		user.Username = username
		return setValue(b, userID, *user)
	})
}

func (d *DB) GetUser(id string) (*UserData, error) {
	if id == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}

	var userData UserData
	err := d.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(d.bucketName)
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("user not found")
		}

		return json.Unmarshal(data, &userData)
	})

	if err != nil {
		return nil, err
	}

	return &userData, nil
}
