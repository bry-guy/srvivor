package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/buildinfo"
	"github.com/gin-gonic/gin"
)

func TestHealthzReportsBuildID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := buildinfo.Commit
	buildinfo.Commit = "0123456789abcdef0123456789abcdef01234567"
	t.Cleanup(func() { buildinfo.Commit = previous })

	router := New(nil).Router()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", recorder.Body.String(), err)
	}
	if body["status"] != "ok" || body["buildId"] != buildinfo.Commit {
		t.Fatalf("healthz body = %v, want status=ok buildId=%s", body, buildinfo.Commit)
	}
}
