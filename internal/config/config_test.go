package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}
	return path
}

const baseYAML = `
server:
  port: 8080
  username: admin
  password: "123456"
  secret: "yaml-secret-value-32chars-xxxxxx"
  session_max_age: 86400
database:
  path: "assethub.db"
`

func TestLoad_NoEnvKeepsYAML(t *testing.T) {
	t.Setenv("ASSETHUB_USERNAME", "")
	t.Setenv("ASSETHUB_PASSWORD", "")
	t.Setenv("ASSETHUB_SECRET", "")
	path := writeTempYAML(t, baseYAML)
	cfg, overridden, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Username != "admin" || cfg.Server.Password != "123456" || cfg.Server.Secret != "yaml-secret-value-32chars-xxxxxx" {
		t.Fatalf("unexpected cfg: %+v", cfg.Server)
	}
	if len(overridden) != 0 {
		t.Fatalf("expected no overrides, got %v", overridden)
	}
}

func TestLoad_EnvOverridesAll(t *testing.T) {
	t.Setenv("ASSETHUB_USERNAME", "ops")
	t.Setenv("ASSETHUB_PASSWORD", "strong-pass")
	t.Setenv("ASSETHUB_SECRET", "env-secret-at-least-32-characters!!")
	path := writeTempYAML(t, baseYAML)
	cfg, overridden, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Username != "ops" || cfg.Server.Password != "strong-pass" || cfg.Server.Secret != "env-secret-at-least-32-characters!!" {
		t.Fatalf("unexpected cfg: %+v", cfg.Server)
	}
	if len(overridden) != 3 {
		t.Fatalf("expected 3 overrides, got %v", overridden)
	}
}

func TestLoad_OnlyPassword(t *testing.T) {
	t.Setenv("ASSETHUB_USERNAME", "")
	t.Setenv("ASSETHUB_PASSWORD", "only-pwd")
	t.Setenv("ASSETHUB_SECRET", "")
	path := writeTempYAML(t, baseYAML)
	cfg, overridden, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Username != "admin" || cfg.Server.Password != "only-pwd" || cfg.Server.Secret != "yaml-secret-value-32chars-xxxxxx" {
		t.Fatalf("unexpected cfg: %+v", cfg.Server)
	}
	if len(overridden) != 1 || overridden[0] != "password" {
		t.Fatalf("expected [password], got %v", overridden)
	}
}

func TestLoad_EmptyEnvDoesNotOverride(t *testing.T) {
	t.Setenv("ASSETHUB_PASSWORD", "")
	path := writeTempYAML(t, baseYAML)
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Password != "123456" {
		t.Fatalf("empty env must not override, got %q", cfg.Server.Password)
	}
}
