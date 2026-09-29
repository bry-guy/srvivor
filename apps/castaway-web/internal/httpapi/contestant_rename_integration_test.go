package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bry-guy/srvivor/apps/castaway-web/internal/db"
	"github.com/bry-guy/srvivor/apps/castaway-web/internal/httpapi"
	"github.com/google/uuid"
)

func TestRenameContestant(t *testing.T) {
	ctx, pool := integrationPool(t)
	defer pool.Close()
	resetDatabase(t, ctx, pool)
	q := db.New(pool)
	instance := createInstanceForTest(t, ctx, q, "Rename", 51)
	other := createInstanceForTest(t, ctx, q, "Other", 50)
	an := createContestantForTest(t, ctx, q, instance.ID, "Thien An Nguyen")
	createContestantForTest(t, ctx, q, instance.ID, "Ana Sani")
	outsider := createContestantForTest(t, ctx, q, other.ID, "Outsider")
	router := httpapi.New(pool, httpapi.WithServiceAuth(httpapi.ServiceAuthConfig{Enabled: true, BearerTokens: []string{"svc"}})).Router()
	rename := func(contestant, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/instances/"+uuid.UUID(instance.ID.Bytes).String()+"/contestants/"+contestant, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer svc")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	anID := uuid.UUID(an.ID.Bytes).String()
	if rec := rename(anID, `{"name":"Thien \"An\" Nguyen"}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `Thien \"An\" Nguyen`) {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if rec := rename(anID, `{"name":"Ana Sani"}`); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate name: %d %s", rec.Code, rec.Body.String())
	}
	if rec := rename(uuid.UUID(outsider.ID.Bytes).String(), `{"name":"Stolen"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("contestant from another instance: %d %s", rec.Code, rec.Body.String())
	}
	if rec := rename(anID, `{"name":"  "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("blank name: %d %s", rec.Code, rec.Body.String())
	}
}
