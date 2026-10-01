package httpapi

import (
	"embed"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

//go:embed site
var siteFiles embed.FS

var siteTemplates = template.Must(template.New("site").ParseFS(siteFiles, "site/*.html"))

type sitePageData struct {
	User               *webSession
	Allowed, Requested bool
	Admin              bool
	InstanceID         string
	Rows               []homeRow
	Profile            *profileView
	Games              []castawordleGameView
	Episodes           []castawordleEpisodeOption
	Game               *castawordleGameView
}

func loginReturnTo(candidate string) string {
	if candidate == "/castawordle" || candidate == "/me" {
		return candidate
	}
	for _, prefix := range []string{"/castawordle/", "/players/"} {
		if suffix, ok := strings.CutPrefix(candidate, prefix); ok {
			if id, err := uuid.Parse(suffix); err == nil {
				return prefix + id.String()
			}
		}
	}
	return "/"
}

func (s *Server) requirePageSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		token, err := c.Cookie(sessionCookie)
		if err != nil {
			token = ""
		}
		ws, err := s.lookupSession(c.Request.Context(), token)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		if ws == nil {
			c.Redirect(http.StatusFound, "/auth/login?next="+url.QueryEscape(loginReturnTo(c.Request.URL.Path)))
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) siteData(c *gin.Context) (sitePageData, bool) {
	data := sitePageData{InstanceID: s.public.InstanceID}
	token, err := c.Cookie(sessionCookie)
	if err != nil {
		token = ""
	}
	data.User, err = s.lookupSession(c.Request.Context(), token)
	if err != nil || data.User == nil {
		c.String(http.StatusUnauthorized, "please log in with Discord")
		return data, false
	}
	id, err := uuid.Parse(s.public.InstanceID)
	if err == nil {
		data.Admin, err = s.isInstanceAdmin(c.Request.Context(), toPGUUID(id), data.User.DiscordUserID)
		if err == nil {
			err = s.pool.QueryRow(c.Request.Context(), `SELECT EXISTS (
				SELECT 1 FROM participants p JOIN instances i ON i.id = p.instance_id
				WHERE i.public_id = $1 AND p.discord_user_id = $2
			)`, id, data.User.DiscordUserID).Scan(&data.Allowed)
			data.Allowed = data.Allowed || data.Admin
		}
	}
	if err != nil {
		c.String(http.StatusInternalServerError, "access check failed")
		return data, false
	}
	if !data.Allowed {
		err = s.pool.QueryRow(c.Request.Context(), `SELECT EXISTS (SELECT 1 FROM access_requests WHERE discord_user_id = $1)`, data.User.DiscordUserID).Scan(&data.Requested)
		if err != nil {
			c.String(http.StatusInternalServerError, "access check failed")
			return data, false
		}
	}
	return data, true
}

func renderSite(c *gin.Context, page string, data sitePageData) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Cache-Control", "no-store")
	if err := siteTemplates.ExecuteTemplate(c.Writer, page, data); err != nil {
		requestLogger.Error("render site", "error", err)
	}
}

func serveSiteAsset(c *gin.Context) {
	name := c.Param("file")
	data, err := siteFiles.ReadFile("site/assets/" + name)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, mime.TypeByExtension(path.Ext(name)), data)
}
