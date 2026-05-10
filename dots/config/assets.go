// Package configassets provides embedded configuration assets for the dots tool.
package configassets

import (
	"bytes"
	"embed"
	"fmt"
	"log/slog"
	"text/template"
)

//go:embed *.tmpl
var embeddedTemplates embed.FS

// RenderTemplate renders the named embedded template with the given key/value data map.
func RenderTemplate(name string, data map[string]string) (string, error) {
	raw, err := embeddedTemplates.ReadFile(name)
	if err != nil {
		slog.Error("RenderTemplate: reading asset", "name", name, "err", err)
		return "", fmt.Errorf("reading asset %s: %w", name, err)
	}
	parsed, err := template.New(name).Parse(string(raw))
	if err != nil {
		slog.Error("RenderTemplate: parsing template", "name", name, "err", err)
		return "", fmt.Errorf("parsing template %s: %w", name, err)
	}
	bb := bytes.NewBuffer(nil)
	if err := parsed.Execute(bb, data); err != nil {
		slog.Error("RenderTemplate: executing template", "name", name, "err", err)
		return "", fmt.Errorf("executing template %s: %w", name, err)
	}
	return bb.String(), nil
}
