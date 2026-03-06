package httpapi

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type openAPIDocument struct {
	Paths map[string]map[string]any `yaml:"paths"`
}

var ginPathParamPattern = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

func TestOpenAPIRoutesMatchRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	specPath := filepath.Join(currentDir(t), "..", "..", "openapi", "openapi.yaml")
	contents, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read openapi spec: %v", err)
	}

	var doc openAPIDocument
	if err := yaml.Unmarshal(contents, &doc); err != nil {
		t.Fatalf("unmarshal openapi spec: %v", err)
	}

	documented := make(map[string]struct{})
	for path, item := range doc.Paths {
		for method := range item {
			if !isHTTPMethod(method) {
				continue
			}
			documented[routeKey(strings.ToUpper(method), path)] = struct{}{}
		}
	}

	router := New(nil).Router()
	registered := make(map[string]struct{})
	for _, route := range router.Routes() {
		if !isHTTPMethod(route.Method) {
			continue
		}
		registered[routeKey(route.Method, normalizeGinPath(route.Path))] = struct{}{}
	}

	missing := diffRouteKeys(documented, registered)
	extra := diffRouteKeys(registered, documented)
	if len(missing) == 0 && len(extra) == 0 {
		return
	}

	var message strings.Builder
	message.WriteString("openapi route parity mismatch")
	if len(missing) > 0 {
		message.WriteString("\nmissing from router:")
		for _, route := range missing {
			message.WriteString("\n  - ")
			message.WriteString(route)
		}
	}
	if len(extra) > 0 {
		message.WriteString("\nmissing from openapi:")
		for _, route := range extra {
			message.WriteString("\n  - ")
			message.WriteString(route)
		}
	}
	message.WriteString("\n\nupdate apps/castaway-web/typespec/main.tsp and regenerate openapi if the router intentionally changed")
	t.Fatal(message.String())
}

func currentDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}
	return filepath.Dir(filename)
}

func normalizeGinPath(path string) string {
	return ginPathParamPattern.ReplaceAllString(path, `{$1}`)
}

func isHTTPMethod(method string) bool {
	switch strings.ToUpper(method) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	default:
		return false
	}
}

func routeKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

func diffRouteKeys(left, right map[string]struct{}) []string {
	diff := make([]string, 0)
	for key := range left {
		if _, ok := right[key]; ok {
			continue
		}
		diff = append(diff, key)
	}
	sort.Strings(diff)
	return diff
}
