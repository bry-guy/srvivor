package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireServiceAuthRejectsMissingBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := New(nil, WithServiceAuth(ServiceAuthConfig{Enabled: true, BearerTokens: []string{"test-token"}}))
	router := gin.New()
	router.Use(server.requireServiceAuth())
	router.GET("/instances", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/instances", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestRequireServiceAuthRejectsInvalidBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := New(nil, WithServiceAuth(ServiceAuthConfig{Enabled: true, BearerTokens: []string{"test-token"}}))
	router := gin.New()
	router.Use(server.requireServiceAuth())
	router.GET("/instances", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/instances", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestRequireServiceAuthAcceptsValidBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := New(nil, WithServiceAuth(ServiceAuthConfig{Enabled: true, BearerTokens: []string{"test-token"}}))
	router := gin.New()
	router.Use(server.requireServiceAuth())
	router.GET("/instances", func(c *gin.Context) {
		principal, ok := ServicePrincipal(c.Request.Context())
		if !ok {
			t.Fatalf("expected service principal in request context")
		}
		if principal != DefaultServicePrincipal {
			t.Fatalf("principal = %q, want %q", principal, DefaultServicePrincipal)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/instances", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestRouterHealthzRemainsUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := New(nil, WithServiceAuth(ServiceAuthConfig{Enabled: true, BearerTokens: []string{"test-token"}}))
	router := server.Router()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}
