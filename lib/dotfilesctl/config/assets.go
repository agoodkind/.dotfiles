package configassets

import (
	"bytes"
	"embed"
	"text/template"
)

//go:embed *.tmpl
var embeddedTemplates embed.FS

func RenderTemplate(name string, data map[string]any) (string, error) {
	raw, err := embeddedTemplates.ReadFile(name)
	if err != nil {
		return "", err
	}
	parsed, err := template.New(name).Parse(string(raw))
	if err != nil {
		return "", err
	}
	bb := bytes.NewBuffer(nil)
	if err := parsed.Execute(bb, data); err != nil {
		return "", err
	}
	return bb.String(), nil
}

