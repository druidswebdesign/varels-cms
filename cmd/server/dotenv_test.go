package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "" +
		"# comment\n" +
		"\n" +
		"PORT=9090\n" +
		"export DB_PATH=./data/custom.db\n" +
		"SIMPLE=value\n" +
		"QUOTED=\"hello world\"\n" +
		"SINGLE='single value'\n" +
		"MALFORMED\n" +
		"=noname\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	for _, k := range []string{"PORT", "DB_PATH", "SIMPLE", "QUOTED", "SINGLE"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	loadDotEnv(path)

	want := map[string]string{
		"PORT":    "9090",
		"DB_PATH": "./data/custom.db",
		"SIMPLE":  "value",
		"QUOTED":  "hello world",
		"SINGLE":  "single value",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestLoadDotEnvDoesNotOverrideProcessEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("PORT=9090\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("PORT", "1234")
	loadDotEnv(path)

	if got := os.Getenv("PORT"); got != "1234" {
		t.Errorf("PORT = %q, want the process environment value 1234", got)
	}
}

func TestLoadDotEnvMissingFileIsNoop(t *testing.T) {
	t.Setenv("SHOULD_NOT_BE_SET", "")
	os.Unsetenv("SHOULD_NOT_BE_SET")
	loadDotEnv(filepath.Join(t.TempDir(), "does-not-exist.env"))
	if _, ok := os.LookupEnv("SHOULD_NOT_BE_SET"); ok {
		t.Error("missing .env set a variable")
	}
}
