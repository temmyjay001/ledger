package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/temmyjay001/ledger-service/internal/config"
	"github.com/temmyjay001/ledger-service/internal/storage"
	"github.com/temmyjay001/ledger-service/internal/storage/queries"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidCredentials = errors.New("Invalid email or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserLocked         = errors.New("account is locked due to too many failed attempts")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token has expired")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidAPIKey      = errors.New("invalid API key")
)

type Service struct {
	db     *storage.DB
	config *config.Config
}

type Claims struct {
	UserID   uuid.UUID  `json:"user_id"`
	Email    string     `json:"email"`
	TenantID *uuid.UUID `json:"tenant_id,omitempty"`
	jwt.RegisteredClaims
}

type APIKeyClaims struct {
	KeyID      uuid.UUID `json:"key_id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	TenantSlug string    `json:"tenant_slug"`
	Scopes     []string  `json:"scopes"`
}

type UserResponse struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	FirstName     string    `json:"first_name"`
	LastName      string    `json:"last_name"`
	EmailVerified bool      `json:"email_verified"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

func NewService(db *storage.DB, config *config.Config) *Service {
	return &Service{
		db:     db,
		config: config,
	}
}

func (s *Service) RegisterUser(ctx context.Context, req RegisterRequest) (*UserResponse, error) {
	// check if email already exists
	_, err := s.db.Queries.GetUserByEmail(ctx, req.Email)
	if err == nil {
		return nil, ErrEmailAlreadyExists
	}

	// Hash Password
	passwordHash, err := s.hashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// create user
	user, err := s.db.Queries.CreateUser(ctx, queries.CreateUserParams{
		Email:        req.Email,
		PasswordHash: passwordHash,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return s.userToResponse(user), nil
}

func (s *Service) LoginUser(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, err := s.db.Queries.GetUserByEmail(ctx, req.Email)

	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if user.LockedUntil.Valid && user.LockedUntil.Time.After(time.Now()) {
		return nil, ErrUserLocked
	}

	valid, err := s.verifyPassword(req.Password, user.PasswordHash)

	if !valid {
		if err := s.db.Queries.IncrementFailedLoginAttempts(ctx, user.ID); err != nil {
			log.Println("failed to increment failed login attempts:", err)
		}
		return nil, ErrInvalidCredentials
	}

	// update last login and reset failed attempt
	if err := s.db.Queries.UpdateUserLastLogin(ctx, user.ID); err != nil {
		log.Println("failed to update last login:", err)
	}

	token, err := s.generateUserToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(time.Hour * 24),
		User:      s.userToResponse(user),
	}, nil
}

func (s *Service) ValidateUserToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// check expiration
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, ErrTokenExpired
	}

	return claims, nil
}

func (s *Service) GenerateAPIKey(ctx context.Context, req CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	// Generate API key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	apiKey := base64.URLEncoding.EncodeToString(keyBytes)
	keyPrefix := apiKey[:8] // First 8 characters for identification
	keyHash := s.hashAPIKey(apiKey)

	// Set expiration if provided
	var expiresAt pgtype.Timestamptz
	if req.ExpiresAt != nil {
		expiresAt = pgtype.Timestamptz{
			Time:  *req.ExpiresAt,
			Valid: true,
		}
	}

	// Create API key record
	apiKeyRecord, err := s.db.Queries.CreateAPIKey(ctx, queries.CreateAPIKeyParams{
		TenantID:  req.TenantID,
		Name:      req.Name,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		Scopes:    req.Scopes,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return &CreateAPIKeyResponse{
		ID:        apiKeyRecord.ID,
		Name:      apiKeyRecord.Name,
		Key:       apiKey, // Only returned once!
		KeyPrefix: keyPrefix,
		Scopes:    apiKeyRecord.Scopes,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: apiKeyRecord.CreatedAt,
	}, nil
}

func (s *Service) ValidateAPIKey(ctx context.Context, apiKey string) (*APIKeyClaims, error) {
	keyHash := s.hashAPIKey(apiKey)

	apiKeyData, err := s.db.Queries.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		return nil, ErrInvalidAPIKey
	}

	// Update last used timestamp (fire and forget)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.db.Queries.UpdateAPIKeyLastUsed(ctx, apiKeyData.ID)
	}()

	return &APIKeyClaims{
		KeyID:      apiKeyData.ID,
		TenantID:   apiKeyData.TenantID,
		TenantSlug: apiKeyData.TenantSlug,
		Scopes:     apiKeyData.Scopes,
	}, nil
}

func (s *Service) generateUserToken(user queries.User) (string, error) {
	claims := &Claims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "ledger-service",
			Subject:   user.ID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

func (s *Service) hashPassword(password string) (string, error) {
	// Generate random salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Hash password with Argon2
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Encode salt and hash
	encodedSalt := base64.StdEncoding.EncodeToString(salt)
	encodedHash := base64.StdEncoding.EncodeToString(hash)

	return fmt.Sprintf("%s:%s", encodedSalt, encodedHash), nil
}

func (s *Service) verifyPassword(password, hashedPassword string) (bool, error) {
	parts := strings.Split(hashedPassword, ":")
	if len(parts) != 2 {
		return false, errors.New("invalid password hash format")
	}

	salt, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return false, err
	}

	expectedHash, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return false, err
	}

	// Hash the provided password with the same salt
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Compare hashes
	return string(hash) == string(expectedHash), nil
}

func (s *Service) hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey + s.config.APIKeySecret))
	return hex.EncodeToString(hash[:])
}

func (s *Service) userToResponse(user queries.User) *UserResponse {
	return &UserResponse{
		ID:            user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		EmailVerified: user.EmailVerified.Bool,
		Status:        string(user.Status.UserStatusEnum),
		CreatedAt:     user.CreatedAt,
	}
}
