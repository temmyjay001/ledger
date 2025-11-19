// internal/auth/service_test.go
package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/temmyjay001/ledger-service/internal/storage/queries"
	"github.com/temmyjay001/ledger-service/internal/config"
)

func TestHashPassword(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:    "test-secret",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)

	password := "MySecurePassword123!"

	hash1, err := service.hashPassword(password)
	assert.NoError(t, err)
	assert.NotEmpty(t, hash1)

	// Verify hash has correct format (salt:hash)
	parts := strings.Split(hash1, ":")
	assert.Len(t, parts, 2, "Hash should have salt and hash parts")

	// Hash should be different each time due to random salt
	hash2, err := service.hashPassword(password)
	assert.NoError(t, err)
	assert.NotEqual(t, hash1, hash2, "Hashes should differ due to different salts")
}

func TestVerifyPassword(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:    "test-secret",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)

	tests := []struct {
		name     string
		password string
	}{
		{"Simple password", "password123"},
		{"Complex password", "MyS3cur3P@ssw0rd!"},
		{"Long password", "ThisIsAVeryLongPasswordWithManyCharacters123!@#"},
		{"Special characters", "!@#$%^&*()_+-={}[]|:;<>?,./"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := service.hashPassword(tt.password)
			assert.NoError(t, err)

			// Correct password should verify
			valid, err := service.verifyPassword(tt.password, hash)
			assert.NoError(t, err)
			assert.True(t, valid, "Correct password should verify")

			// Incorrect password should not verify
			valid, err = service.verifyPassword("wrongpassword", hash)
			assert.NoError(t, err)
			assert.False(t, valid, "Incorrect password should not verify")
		})
	}
}

func TestVerifyPassword_InvalidFormat(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:    "test-secret",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)

	tests := []struct {
		name        string
		hashedPass  string
		expectError bool
	}{
		{
			name:        "Invalid format - no colon",
			hashedPass:  "invalidhash",
			expectError: true,
		},
		{
			name:        "Invalid format - empty",
			hashedPass:  "",
			expectError: true,
		},
		{
			name:        "Invalid format - only colon",
			hashedPass:  ":",
			expectError: false, // Will verify but return false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := service.verifyPassword("password", tt.hashedPass)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.False(t, valid)
			}
		})
	}
}

func TestGenerateUserToken(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:    "test-jwt-secret-key",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)

	userID := uuid.New()
	email := "test@example.com"

	user := queries.User{
		ID:    userID,
		Email: email,
	}

	token, err := service.generateUserToken(user)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Parse and validate token
	parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.JWTSecret), nil
	})

	assert.NoError(t, err)
	assert.True(t, parsedToken.Valid)

	claims, ok := parsedToken.Claims.(*Claims)
	assert.True(t, ok)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
	assert.Equal(t, "ledger-service", claims.Issuer)
	assert.NotNil(t, claims.ExpiresAt)
	assert.NotNil(t, claims.IssuedAt)
}

func TestValidateUserToken(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:    "test-jwt-secret-key",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)

	userID := uuid.New()
	email := "test@example.com"

	user := queries.User{
		ID:    userID,
		Email: email,
	}

	// Generate valid token
	token, err := service.generateUserToken(user)
	assert.NoError(t, err)

	// Validate token
	claims, err := service.ValidateUserToken(token)
	assert.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, userID, claims.UserID)
	assert.Equal(t, email, claims.Email)
}

func TestValidateUserToken_Invalid(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:    "test-jwt-secret-key",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)

	tests := []struct {
		name  string
		token string
	}{
		{"Empty token", ""},
		{"Invalid format", "not.a.valid.token"},
		{"Random string", "randomstring"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := service.ValidateUserToken(tt.token)
			assert.Error(t, err)
			assert.Nil(t, claims)
		})
	}
}

func TestValidateUserToken_Expired(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:    "test-jwt-secret-key",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)

	userID := uuid.New()

	// Create expired token
	claims := &Claims{
		UserID: userID,
		Email:  "test@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // Expired 1 hour ago
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "ledger-service",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
	assert.NoError(t, err)

	// Validate expired token
	validatedClaims, err := service.ValidateUserToken(tokenString)
	assert.Error(t, err)
	assert.Nil(t, validatedClaims)
}

func TestHashAPIKey(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:    "test-secret",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)

	apiKey := "test-api-key-12345"

	hash := service.hashAPIKey(apiKey)
	assert.NotEmpty(t, hash)
	assert.Equal(t, 64, len(hash), "SHA256 hash should be 64 hex characters")

	// Verify it's a valid hex string
	_, err := hex.DecodeString(hash)
	assert.NoError(t, err)

	// Same key should produce same hash
	hash2 := service.hashAPIKey(apiKey)
	assert.Equal(t, hash, hash2)

	// Different key should produce different hash
	hash3 := service.hashAPIKey("different-key")
	assert.NotEqual(t, hash, hash3)
}

func TestHashAPIKey_Deterministic(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:    "test-secret",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)

	apiKey := "consistent-key"

	// Hash multiple times
	hashes := make([]string, 10)
	for i := 0; i < 10; i++ {
		hashes[i] = service.hashAPIKey(apiKey)
	}

	// All hashes should be identical
	for i := 1; i < len(hashes); i++ {
		assert.Equal(t, hashes[0], hashes[i], "API key hash should be deterministic")
	}
}

func TestValidateScopes(t *testing.T) {
	tests := []struct {
		name   string
		scopes []string
		want   bool
	}{
		{
			name:   "Valid single scope",
			scopes: []string{"transactions:read"},
			want:   true,
		},
		{
			name:   "Valid multiple scopes",
			scopes: []string{"transactions:read", "accounts:write", "balances:read"},
			want:   true,
		},
		{
			name:   "All valid scopes",
			scopes: ValidScopes,
			want:   true,
		},
		{
			name:   "Invalid scope",
			scopes: []string{"invalid:scope"},
			want:   false,
		},
		{
			name:   "Mix of valid and invalid",
			scopes: []string{"transactions:read", "invalid:scope"},
			want:   false,
		},
		{
			name:   "Empty scopes",
			scopes: []string{},
			want:   true, // Empty is valid (no invalid scopes)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateScopes(tt.scopes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func BenchmarkHashPassword(b *testing.B) {
	cfg := &config.Config{
		JWTSecret:    "test-secret",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)
	password := "MySecurePassword123!"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.hashPassword(password)
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	cfg := &config.Config{
		JWTSecret:    "test-secret",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)
	password := "MySecurePassword123!"
	hash, _ := service.hashPassword(password)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.verifyPassword(password, hash)
	}
}

func BenchmarkHashAPIKey(b *testing.B) {
	cfg := &config.Config{
		JWTSecret:    "test-secret",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)
	apiKey := "test-api-key-12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.hashAPIKey(apiKey)
	}
}

// Mock user for testing
type MockUser struct {
	ID    uuid.UUID
	Email string
}

// Test helper to create expected hash
func expectedAPIKeyHash(apiKey, secret string) string {
	hash := sha256.Sum256([]byte(apiKey + secret))
	return hex.EncodeToString(hash[:])
}

func TestHashAPIKey_MatchesExpected(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:    "test-secret",
		APIKeySecret: "test-api-secret",
	}
	service := NewService(nil, cfg)

	apiKey := "test-key"
	expected := expectedAPIKeyHash(apiKey, cfg.APIKeySecret)
	actual := service.hashAPIKey(apiKey)

	assert.Equal(t, expected, actual, "Hash should match expected SHA256 calculation")
}
