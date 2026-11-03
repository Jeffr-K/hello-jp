package prompts

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates
var templateFiles embed.FS

type Renderer struct{ templates *template.Template }

func NewRenderer() (*Renderer, error) {
	templates, err := template.New("prompts").Option("missingkey=error").ParseFS(templateFiles, "templates/**/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("parse prompt templates: %w", err)
	}

	return &Renderer{templates: templates}, nil
}

func (r *Renderer) Render(ctx context.Context, name string, data any) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var output strings.Builder
	if err := r.templates.ExecuteTemplate(&output, name, data); err != nil {
		return "", fmt.Errorf("render prompt %q: %w", name, err)
	}

	return output.String(), nil
}
