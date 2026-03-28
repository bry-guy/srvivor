package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/gin-gonic/gin"
)

const (
	DefaultServicePrincipal = "castaway-discord-bot"
	discordUserIDHeader     = "X-Discord-User-ID"
)

type ServiceAuthConfig struct {
	Enabled             bool
	BearerTokens        []string
	Principal           string
	DiscordAdminUserIDs []string
}

type serviceAuthContextKey struct{}

func ServicePrincipal(ctx context.Context) (string, bool) {
	principal, ok := ctx.Value(serviceAuthContextKey{}).(string)
	if !ok || strings.TrimSpace(principal) == "" {
		return "", false
	}
	return principal, true
}

func normalizeServiceAuthConfig(cfg ServiceAuthConfig) ServiceAuthConfig {
	cfg.Principal = strings.TrimSpace(cfg.Principal)
	if cfg.Principal == "" {
		cfg.Principal = DefaultServicePrincipal
	}

	tokens := make([]string, 0, len(cfg.BearerTokens))
	seen := make(map[string]struct{}, len(cfg.BearerTokens))
	for _, token := range cfg.BearerTokens {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		tokens = append(tokens, trimmed)
	}
	cfg.BearerTokens = tokens

	adminUserIDs := make([]string, 0, len(cfg.DiscordAdminUserIDs))
	seen = make(map[string]struct{}, len(cfg.DiscordAdminUserIDs))
	for _, userID := range cfg.DiscordAdminUserIDs {
		trimmed := strings.TrimSpace(userID)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		adminUserIDs = append(adminUserIDs, trimmed)
	}
	cfg.DiscordAdminUserIDs = adminUserIDs
	return cfg
}

func (s *Server) requireServiceAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.serviceAuth.Enabled {
			c.Next()
			return
		}

		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(authorization, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
			return
		}

		token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
			return
		}
		if _, ok := s.serviceAuthBearerTokens[token]; !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
			return
		}

		ctx := context.WithValue(c.Request.Context(), serviceAuthContextKey{}, s.serviceAuth.Principal)
		c.Request = c.Request.WithContext(ctx)
		c.Set("service_principal", s.serviceAuth.Principal)
		c.Next()
	}
}

func discordUserIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.Header.Get(discordUserIDHeader))
}

func (s *Server) isDiscordAdmin(discordUserID string) bool {
	if strings.TrimSpace(discordUserID) == "" {
		return false
	}
	_, ok := s.serviceAuthDiscordAdminUserIDs[strings.TrimSpace(discordUserID)]
	return ok
}

func (s *Server) canViewSecretParticipantData(discordUserID string, participant db.GetParticipantRow) bool {
	if s.isDiscordAdmin(discordUserID) {
		return true
	}
	if !participant.DiscordUserID.Valid {
		return false
	}
	return strings.TrimSpace(participant.DiscordUserID.String) != "" && participant.DiscordUserID.String == strings.TrimSpace(discordUserID)
}
