package auth

import (
	"testing"
	"time"
)

func TestNewJWTConfig(t *testing.T) {
	secretKey := []byte("test_secret")
	duration := 1 * time.Hour

	config := NewJWTConfig(secretKey, duration)

	if string(config.SecretKey) != string(secretKey) {
		t.Errorf("Expected secret key %s, got %s", secretKey, config.SecretKey)
	}

	if config.TokenDuration != duration {
		t.Errorf("Expected token duration %v, got %v", duration, config.TokenDuration)
	}
}

func TestDefaultJWTConfig(t *testing.T) {
	config := DefaultJWTConfig()

	if string(config.SecretKey) != "my_secret_key" {
		t.Errorf("Expected default secret key 'my_secret_key', got %s", config.SecretKey)
	}

	if config.TokenDuration != 24*time.Hour {
		t.Errorf("Expected default token duration 24h, got %v", config.TokenDuration)
	}
}

func TestGenerateToken(t *testing.T) {
	config := NewJWTConfig([]byte("test_secret"), 1*time.Hour)

	// Test successful token generation
	token, err := config.GenerateToken("testuser", "123", true, false)
	if err != nil {
		t.Errorf("Failed to generate token: %v", err)
	}
	if token == "" {
		t.Error("Generated token is empty")
	}

	// Test missing username
	_, err = config.GenerateToken("", "123", true, false)
	if err == nil {
		t.Error("Expected error for empty username, got nil")
	}

	// Test missing ID
	_, err = config.GenerateToken("testuser", "", true, false)
	if err == nil {
		t.Error("Expected error for empty ID, got nil")
	}
}

func TestValidateToken(t *testing.T) {
	config := NewJWTConfig([]byte("test_secret"), 1*time.Hour)

	// Generate a valid token
	tokenString, _ := config.GenerateToken("testuser", "123", true, false)

	// Test successful validation
	claims, err := config.ValidateToken(tokenString)
	if err != nil {
		t.Errorf("Failed to validate token: %v", err)
	}
	if claims.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", claims.Username)
	}
	if claims.ID != "123" {
		t.Errorf("Expected ID '123', got '%s'", claims.ID)
	}
	if claims.IsStudent != true {
		t.Errorf("Expected IsStudent true, got %v", claims.IsStudent)
	}
	if claims.IsTeacher != false {
		t.Errorf("Expected IsTeacher false, got %v", claims.IsTeacher)
	}

	// Test empty token
	_, err = config.ValidateToken("")
	if err == nil {
		t.Error("Expected error for empty token, got nil")
	}

	// Test invalid token format
	_, err = config.ValidateToken("invalid.token.format")
	if err == nil {
		t.Error("Expected error for invalid token format, got nil")
	}

	// Test token with wrong signature
	wrongConfig := NewJWTConfig([]byte("wrong_secret"), 1*time.Hour)
	_, err = wrongConfig.ValidateToken(tokenString)
	if err == nil {
		t.Error("Expected error for token with wrong signature, got nil")
	}

	// Test expired token
	expiredConfig := NewJWTConfig([]byte("test_secret"), -1*time.Hour) // Negative duration for expired token
	expiredToken, _ := expiredConfig.GenerateToken("testuser", "123", true, false)
	_, err = config.ValidateToken(expiredToken)
	if err == nil {
		t.Error("Expected error for expired token, got nil")
	}
}

func TestExtractUserInfo(t *testing.T) {
	config := NewJWTConfig([]byte("test_secret"), 1*time.Hour)

	// Generate a valid token
	tokenString, _ := config.GenerateToken("testuser", "123", false, true)

	// Test successful extraction
	username, id, err := config.ExtractUserInfo(tokenString)
	if err != nil {
		t.Errorf("Failed to extract user info: %v", err)
	}
	if username != "testuser" {
		t.Errorf("Expected username 'testuser', got %s", username)
	}
	if id != "123" {
		t.Errorf("Expected ID '123', got %s", id)
	}

	// Test invalid token
	_, _, err = config.ExtractUserInfo("invalid.token")
	if err == nil {
		t.Error("Expected error for invalid token, got nil")
	}
}

func TestTokenExpiration(t *testing.T) {
	// Create a config with a very short expiration
	config := NewJWTConfig([]byte("test_secret"), 1*time.Second)

	// Generate token
	token, _ := config.GenerateToken("testuser", "123", true, false)

	// Verify token is valid initially
	_, err := config.ValidateToken(token)
	if err != nil {
		t.Errorf("Token should be valid initially: %v", err)
	}

	// Wait for token to expire
	time.Sleep(2 * time.Second)

	// Verify token is now invalid
	_, err = config.ValidateToken(token)
	if err == nil {
		t.Error("Expected error for expired token, got nil")
	}
}

func TestExtractUserInfoWithRoles(t *testing.T) {
	config := NewJWTConfig([]byte("test_secret"), 1*time.Hour)

	// Generate a valid token with both roles
	tokenString, _ := config.GenerateToken("testuser", "123", true, true)

	// Test successful extraction
	userClaim, err := config.ExtractUserInfoWithRoles(tokenString)
	if err != nil {
		t.Errorf("Failed to extract user info with roles: %v", err)
	}
	if userClaim.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", userClaim.Username)
	}
	if userClaim.ID != "123" {
		t.Errorf("Expected ID '123', got '%s'", userClaim.ID)
	}
	if userClaim.IsStudent != true {
		t.Errorf("Expected IsStudent true, got %v", userClaim.IsStudent)
	}
	if userClaim.IsTeacher != true {
		t.Errorf("Expected IsTeacher true, got %v", userClaim.IsTeacher)
	}

	// Test with invalid token
	_, err = config.ExtractUserInfoWithRoles("")
	if err == nil {
		t.Error("Expected error for empty token, got nil")
	}
}
