package template

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"
)

type Templator struct {
	Git Git
	Env map[string]string
}

type Git struct {
	Revision string
}

func New(gitRevision string) *Templator {
	data := os.Environ()
	envMap := make(map[string]string)
	for _, val := range data {
		splits := strings.SplitN(val, "=", 2)
		key := splits[0]
		value := splits[1]
		envMap[key] = value
	}
	return &Templator{
		Git: Git{strings.ReplaceAll(gitRevision, "/", "_")},
		Env: envMap,
	}
}

func (t Templator) Exec(text string) (string, error) {
	return exec(text, t)
}

// ExecEnv exposes only .Env so that connection settings cannot depend on the revision.
func (t Templator) ExecEnv(text string) (string, error) {
	return exec(text, struct{ Env map[string]string }{t.Env})
}

func exec(text string, data interface{}) (string, error) {
	if text == "" {
		return "", nil
	}
	engine := template.New("Templating").Option("missingkey=error")
	tmpl, err := engine.Parse(text)
	if err != nil {
		return "", fmt.Errorf("Error to parse template: %w", err)
	}
	val := bytes.Buffer{}
	if err := tmpl.Execute(&val, data); err != nil {
		return "", fmt.Errorf("Error to parse template: %w", err)
	}
	return val.String(), nil
}
