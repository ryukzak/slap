package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDB(t *testing.T) (*DB, string) {
	tempDir, err := os.MkdirTemp("", "slap-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := NewDB(dbPath, "")
	if err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("Failed to create test database: %v", err)
	}

	return db, tempDir
}

func cleanupTestDB(db *DB, tempDir string) {
	if db != nil {
		_ = db.Close()
	}
	if tempDir != "" {
		_ = os.RemoveAll(tempDir)
	}
}

func TestSaveAndGetUser(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	user := &UserData{
		ID:       "123",
		Username: "testuser",
	}

	err := db.SaveUser(user)
	if err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	retrieved, err := db.GetUser("123")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if retrieved.ID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, retrieved.ID)
	}

	if retrieved.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, retrieved.Username)
	}

	if retrieved.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	// Check that Journals is initialized
	if retrieved.Journals == nil {
		t.Error("Expected Journals to be initialized")
	}

	// Check that LessonIDs is initialized
	if retrieved.LessonIDs == nil {
		t.Error("Expected LessonIDs to be initialized")
	}

	// Check that role flags are preserved
	if retrieved.IsStudent != false {
		t.Error("Expected IsStudent to be false by default")
	}
	if retrieved.IsTeacher != false {
		t.Error("Expected IsTeacher to be false by default")
	}
}

func TestGetNonExistentUser(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	_, err := db.GetUser("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent user")
	}
}

func TestEmptyIDValidation(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Test saving with empty ID
	err := db.SaveUser(&UserData{Username: "noID"})
	if err == nil {
		t.Error("Expected error when saving user with empty ID")
	}

	// Test updating an existing user's username
	user := &UserData{
		ID:       "update_test",
		Username: "original_name",
	}
	err = db.SaveUser(user)
	if err != nil {
		t.Fatalf("Failed to save initial user: %v", err)
	}

	// Update username
	updatedUser := &UserData{
		ID:       "update_test",
		Username: "updated_name",
	}
	err = db.SaveUser(updatedUser)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Get the user and verify username was updated
	retrieved, err := db.GetUser("update_test")
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}
	if retrieved.Username != "updated_name" {
		t.Errorf("Expected username to be updated to %s, got %s",
			"updated_name", retrieved.Username)
	}

	// Test getting with empty ID
	_, err = db.GetUser("")
	if err == nil {
		t.Error("Expected error when getting user with empty ID")
	}
}

func TestUserRoleFlags(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer cleanupTestDB(db, tempDir)

	// Test saving user with role flags
	user := &UserData{
		ID:        "teacher1",
		Username:  "teacheruser",
		IsStudent: false,
		IsTeacher: true,
	}

	err := db.SaveUser(user)
	if err != nil {
		t.Fatalf("Failed to save user: %v", err)
	}

	// Retrieve and verify
	retrieved, err := db.GetUser("teacher1")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if retrieved.IsStudent != false {
		t.Errorf("Expected IsStudent false, got %v", retrieved.IsStudent)
	}
	if retrieved.IsTeacher != true {
		t.Errorf("Expected IsTeacher true, got %v", retrieved.IsTeacher)
	}

	// Test updating role flags
	user.IsStudent = true
	err = db.SaveUser(user)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	retrieved, err = db.GetUser("teacher1")
	if err != nil {
		t.Fatalf("Failed to get updated user: %v", err)
	}

	if retrieved.IsStudent != true {
		t.Errorf("Expected IsStudent true after update, got %v", retrieved.IsStudent)
	}
	if retrieved.IsTeacher != true {
		t.Errorf("Expected IsTeacher true after update, got %v", retrieved.IsTeacher)
	}
}
