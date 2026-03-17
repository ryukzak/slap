package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ryukzak/slap/src/storage"
)

// JWTConfig holds the configuration for JWT operations
type JWTConfig struct {
	SecretKey     []byte
	TokenDuration time.Duration
}

// UserClaims represents the JWT claims containing user information
type UserClaims struct {
	Username  string         `json:"username"`
	ID        storage.UserID `json:"id"`
	IsStudent bool           `json:"is_student"`
	IsTeacher bool           `json:"is_teacher"`
	jwt.RegisteredClaims
}

// NewJWTConfig creates a new JWT configuration with default values
func NewJWTConfig(secretKey []byte, tokenDuration time.Duration) *JWTConfig {
	return &JWTConfig{
		SecretKey:     secretKey,
		TokenDuration: tokenDuration,
	}
}

// DefaultJWTConfig creates a JWT configuration with default values
func DefaultJWTConfig() *JWTConfig {
	return &JWTConfig{
		SecretKey:     []byte("my_secret_key"), // In production, use environment variables
		TokenDuration: 24 * time.Hour,
	}
}

// GenerateToken creates a new JWT token for a user
func (c *JWTConfig) GenerateToken(username, id string, isStudent, isTeacher bool) (string, error) {
	if username == "" || id == "" {
		return "", errors.New("username and ID are required")
	}

	// Create the JWT claims
	expirationTime := time.Now().Add(c.TokenDuration)
	claims := &UserClaims{
		Username:  username,
		ID:        id,
		IsStudent: isStudent,
		IsTeacher: isTeacher,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with our secret key
	tokenString, err := token.SignedString(c.SecretKey)
	if err != nil {
		return "", errors.New("failed to sign token")
	}

	return tokenString, nil
}

// ValidateToken validates and parses the given token
func (c *JWTConfig) ValidateToken(tokenString string) (*UserClaims, error) {
	if tokenString == "" {
		return nil, errors.New("token is required")
	}

	// Parse and validate the token
	claims := &UserClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return c.SecretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// ExtractUserInfo extracts username and ID from a validated token
func (c *JWTConfig) ExtractUserInfo(tokenString string) (string, string, error) {
	claims, err := c.ValidateToken(tokenString)
	if err != nil {
		return "", "", err
	}

	return claims.Username, claims.ID, nil
}

func (c *JWTConfig) ExtractUserInfoWithRoles(tokenString string) (*UserClaims, error) {
	claims, err := c.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	return claims, nil
}
