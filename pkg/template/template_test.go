package template

import "testing"

func TestTemplator_ExecEnv(t *testing.T) {
	tmpl := &Templator{Git: Git{"rev"}, Env: map[string]string{"A": "a"}}
	if got, err := tmpl.ExecEnv("{{.Env.A}}"); err != nil || got != "a" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := tmpl.ExecEnv("{{.Git.Revision}}"); err == nil {
		t.Error("expected error for .Git.Revision")
	}
	if _, err := tmpl.ExecEnv("{{.Env.B}}"); err == nil {
		t.Error("expected error for missing env")
	}
	if got, err := tmpl.Exec("{{.Git.Revision}}"); err != nil || got != "rev" {
		t.Errorf("Exec got %q, %v", got, err)
	}
}
