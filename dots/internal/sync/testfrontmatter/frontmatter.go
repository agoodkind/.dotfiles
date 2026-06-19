// Package testfrontmatter provides helpers for asserting on YAML front matter in tests.
package testfrontmatter

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// ParseMap unmarshals YAML front matter from rendered rule content.
func ParseMap(t *testing.T, content string) map[string]any {
	t.Helper()
	if !strings.HasPrefix(content, "---\n") {
		t.Fatalf("expected front matter in content:\n%s", content)
	}
	endMarker := "\n---\n"
	frontmatterEnd := strings.Index(content[4:], endMarker)
	if frontmatterEnd == -1 {
		t.Fatalf("expected closing front matter delimiter in content:\n%s", content)
	}
	rawFrontmatter := content[4 : 4+frontmatterEnd]
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(rawFrontmatter), &metadata); err != nil {
		t.Fatalf("unmarshaling front matter: %v\n%s", err, rawFrontmatter)
	}
	return metadata
}

// StringField returns a string front matter field from rendered rule content.
func StringField(t *testing.T, content string, key string) string {
	t.Helper()
	metadata := ParseMap(t, content)
	value, ok := metadata[key].(string)
	if !ok {
		t.Fatalf("front matter key %q missing or not a string in %#v", key, metadata)
	}
	return value
}

// StringSliceField returns a string slice front matter field from rendered rule content.
func StringSliceField(t *testing.T, content string, key string) []string {
	t.Helper()
	metadata := ParseMap(t, content)
	rawValues, ok := metadata[key].([]any)
	if !ok {
		t.Fatalf("front matter key %q missing or not a string slice in %#v", key, metadata)
	}
	values := make([]string, 0, len(rawValues))
	for _, rawValue := range rawValues {
		stringValue, ok := rawValue.(string)
		if !ok {
			t.Fatalf("front matter key %q contains non-string value in %#v", key, metadata)
		}
		values = append(values, stringValue)
	}
	return values
}
