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
	Paths      map[string]map[string]any `yaml:"paths"`
	Components struct {
		Schemas map[string]openAPISchema `yaml:"schemas"`
	} `yaml:"components"`
}

type openAPISchema struct {
	Ref        string                   `yaml:"$ref"`
	Type       string                   `yaml:"type"`
	Format     string                   `yaml:"format"`
	Required   []string                 `yaml:"required"`
	Properties map[string]openAPISchema `yaml:"properties"`
	Items      *openAPISchema           `yaml:"items"`
}

var ginPathParamPattern = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

func TestOpenAPIRoutesMatchRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	doc := loadOpenAPIDocument(t)
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

func TestOpenAPIDescribesInstanceEpisodeFields(t *testing.T) {
	doc := loadOpenAPIDocument(t)

	instance, ok := doc.Components.Schemas["Instance"]
	if !ok {
		t.Fatal("Instance schema is missing")
	}
	currentEpisode, ok := instance.Properties["current_episode"]
	if !ok || currentEpisode.Ref != "#/components/schemas/InstanceEpisodeBrief" {
		t.Fatalf("unexpected Instance.current_episode schema: %#v", currentEpisode)
	}
	if hasRequiredField(instance, "current_episode") {
		t.Fatal("Instance.current_episode must remain optional")
	}

	response, ok := doc.Components.Schemas["GetInstanceResponse"]
	if !ok {
		t.Fatal("GetInstanceResponse schema is missing")
	}
	episodes, ok := response.Properties["episodes"]
	if !ok || episodes.Type != "array" || episodes.Items == nil || episodes.Items.Ref != "#/components/schemas/InstanceEpisodeBrief" {
		t.Fatalf("unexpected GetInstanceResponse.episodes schema: %#v", episodes)
	}
	if !hasRequiredField(response, "episodes") {
		t.Fatal("GetInstanceResponse.episodes must be required")
	}

	episode, ok := doc.Components.Schemas["InstanceEpisodeBrief"]
	if !ok {
		t.Fatal("InstanceEpisodeBrief schema is missing")
	}
	for name, expected := range map[string]struct {
		typeName string
		format   string
	}{
		"id":             {typeName: "string"},
		"episode_number": {typeName: "integer", format: "int32"},
		"label":          {typeName: "string"},
		"airs_at":        {typeName: "string", format: "date-time"},
	} {
		property, ok := episode.Properties[name]
		if !ok || property.Type != expected.typeName || property.Format != expected.format {
			t.Fatalf("unexpected InstanceEpisodeBrief.%s schema: %#v", name, property)
		}
		if !hasRequiredField(episode, name) {
			t.Errorf("InstanceEpisodeBrief.%s must be required", name)
		}
	}
}

func hasRequiredField(schema openAPISchema, name string) bool {
	for _, field := range schema.Required {
		if field == name {
			return true
		}
	}
	return false
}

func loadOpenAPIDocument(t *testing.T) openAPIDocument {
	t.Helper()
	specPath := filepath.Join(currentDir(t), "..", "..", "openapi", "openapi.yaml")
	contents, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read openapi spec: %v", err)
	}

	var doc openAPIDocument
	if err := yaml.Unmarshal(contents, &doc); err != nil {
		t.Fatalf("unmarshal openapi spec: %v", err)
	}
	return doc
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
