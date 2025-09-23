// internal/auth/middleware.go
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/temmyjay001/ledger-service/pkg/api"
)

type Middleware struct {
	authService *Service
}

func NewMiddleware(authService *Service) *Middleware {
	return &Middleware{
		authService: authService,
	}
}

// UserAuthMiddleware validates JWT tokens for user authentication
func (m *Middleware) UserAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		token := m.extractTokenFromHeader(r)
		if token == "" {
			m.writeUnauthorizedResponse(w, "missing authorization token")
			return
		}

		// Validate token
		claims, err := m.authService.ValidateUserToken(token)
		if err != nil {
			m.writeUnauthorizedResponse(w, "invalid token")
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), UserContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// APIKeyAuthMiddleware validates API keys for tenant-scoped operations
func (m *Middleware) APIKeyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract API key from Authorization header
		apiKey := m.extractAPIKeyFromHeader(r)
		if apiKey == "" {
			m.writeUnauthorizedResponse(w, "missing API key")
			return
		}

		// Validate API key
		claims, err := m.authService.ValidateAPIKey(r.Context(), apiKey)
		if err != nil {
			m.writeUnauthorizedResponse(w, "invalid API key")
			return
		}

		// Add claims to request context
		ctx := context.WithValue(r.Context(), APIKeyContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireScopes middleware to check if API key has required scopes
func (m *Middleware) RequireScopes(requiredScopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get API key claims from context
			claims, ok := GetAPIKeyClaims(r.Context())
			if !ok {
				m.writeForbiddenResponse(w, "API key required")
				return
			}

			// Check if API key has required scopes
			if !m.hasRequiredScopes(claims.Scopes, requiredScopes) {
				m.writeForbiddenResponse(w, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Helper methods

func (m *Middleware) extractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Expected format: "Bearer <token>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return parts[1]
}

func (m *Middleware) extractAPIKeyFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Expected format: "Bearer <api_key>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}

	return parts[1]
}

func (m *Middleware) hasRequiredScopes(userScopes, requiredScopes []string) bool {
	// Create a map of user scopes for efficient lookup
	userScopeMap := make(map[string]bool)
	for _, scope := range userScopes {
		userScopeMap[scope] = true
	}

	// Check if all required scopes are present
	for _, requiredScope := range requiredScopes {
		if !userScopeMap[requiredScope] {
			return false
		}
	}

	return true
}

func (m *Middleware) writeUnauthorizedResponse(w http.ResponseWriter, message string) {
	api.WriteUnauthorizedResponse(w, message)
}

func (m *Middleware) writeForbiddenResponse(w http.ResponseWriter, message string) {
	api.WriteForbiddenResponse(w, message)
}

// Helper functions to extract claims from context

func GetUserClaims(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(UserContextKey).(*Claims)
	return claims, ok
}

func GetAPIKeyClaims(ctx context.Context) (*APIKeyClaims, bool) {
	claims, ok := ctx.Value(APIKeyContextKey).(*APIKeyClaims)
	return claims, ok
}
