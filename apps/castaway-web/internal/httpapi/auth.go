package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	DefaultServicePrincipal = "castaway-discord-bot"
	discordUserIDHeader     = "X-Discord-User-ID"
)

type ServiceAuthConfig struct {
	Enabled      bool
	BearerTokens []string
	Principal    string
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

func (s *Server) isInstanceAdmin(ctx context.Context, instanceID pgtype.UUID, discordUserID string) (bool, error) {
	if strings.TrimSpace(discordUserID) == "" {
		return false, nil
	}
	return s.queries.IsInstanceAdmin(ctx, db.IsInstanceAdminParams{
		InstanceID:    instanceID,
		DiscordUserID: strings.TrimSpace(discordUserID),
	})
}

func (s *Server) canViewSecretParticipantData(ctx context.Context, instanceID pgtype.UUID, discordUserID string, participant db.GetParticipantRow) (bool, error) {
	if participant.DiscordUserID.Valid {
		if strings.TrimSpace(participant.DiscordUserID.String) != "" && participant.DiscordUserID.String == strings.TrimSpace(discordUserID) {
			return true, nil
		}
	}
	return s.isInstanceAdmin(ctx, instanceID, discordUserID)
}
