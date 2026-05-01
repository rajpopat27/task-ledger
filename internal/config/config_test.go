package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func envSnapshot(t *testing.T) func() {
	t.Helper()
	saved := make(map[string]string)
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "TL_") || strings.HasPrefix(env, "TASK_LEDGER_") {
			parts := strings.SplitN(env, "=", 2)
			saved[parts[0]] = os.Getenv(parts[0])
			_ = os.Unsetenv(parts[0])
		}
	}
	return func() {
		for _, env := range os.Environ() {
			if strings.HasPrefix(env, "TL_") || strings.HasPrefix(env, "TASK_LEDGER_") {
				parts := strings.SplitN(env, "=", 2)
				_ = os.Unsetenv(parts[0])
			}
		}
		for key, val := range saved {
			_ = os.Setenv(key, val)
		}
	}
}

func TestInitializeDefaults(t *testing.T) {
	restore := envSnapshot(t)
	defer restore()

	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() returned error: %v", err)
	}
	if v == nil {
		t.Fatal("viper instance is nil after Initialize")
	}

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"json", GetBool("json"), false},
		{"readonly", GetBool("readonly"), false},
		{"actor", GetString("actor"), ""},
		{"create.require-description", GetBool("create.require-description"), false},
		{"validation.on-create", GetString("validation.on-create"), "none"},
		{"hierarchy.max-depth", GetInt("hierarchy.max-depth"), 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestEnvironmentBinding(t *testing.T) {
	restore := envSnapshot(t)
	defer restore()

	tests := []struct {
		envKey string
		envVal string
		key    string
		got    func(string) interface{}
		want   interface{}
	}{
		{"TL_JSON", "true", "json", func(k string) interface{} { return GetBool(k) }, true},
		{"TL_READONLY", "true", "readonly", func(k string) interface{} { return GetBool(k) }, true},
		{"TL_ACTOR", "tl-user", "actor", func(k string) interface{} { return GetString(k) }, "tl-user"},
		{"TASK_LEDGER_ACTOR", "taskledger-user", "actor", func(k string) interface{} { return GetString(k) }, "taskledger-user"},
	}

	for _, tt := range tests {
		t.Run(tt.envKey, func(t *testing.T) {
			restoreEach := envSnapshot(t)
			defer restoreEach()

			_ = os.Setenv(tt.envKey, tt.envVal)
			if err := Initialize(); err != nil {
				t.Fatalf("Initialize() returned error: %v", err)
			}
			if got := tt.got(tt.key); got != tt.want {
				t.Fatalf("%s=%s produced %v, want %v", tt.envKey, tt.envVal, got, tt.want)
			}
		})
	}
}

func TestProjectConfigFile(t *testing.T) {
	restore := envSnapshot(t)
	defer restore()

	tmpDir := t.TempDir()
	taskLedgerDir := filepath.Join(tmpDir, ".task-ledger")
	if err := os.MkdirAll(taskLedgerDir, 0750); err != nil {
		t.Fatalf("mkdir .task-ledger: %v", err)
	}
	configPath := filepath.Join(taskLedgerDir, "config.yaml")
	configContent := `
json: true
readonly: true
actor: config-user
create:
  require-description: true
validation:
  on-create: error
hierarchy:
  max-depth: 5
directory:
  labels:
    packages/api: backend
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Chdir(tmpDir)

	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() returned error: %v", err)
	}

	if !GetBool("json") || !GetBool("readonly") || !GetBool("create.require-description") {
		t.Fatalf("boolean config values were not loaded")
	}
	if got := GetString("actor"); got != "config-user" {
		t.Fatalf("actor = %q, want config-user", got)
	}
	if got := GetString("validation.on-create"); got != "error" {
		t.Fatalf("validation.on-create = %q, want error", got)
	}
	if got := GetInt("hierarchy.max-depth"); got != 5 {
		t.Fatalf("hierarchy.max-depth = %d, want 5", got)
	}
	if got := GetStringMapString("directory.labels"); got["packages/api"] != "backend" {
		t.Fatalf("directory.labels = %#v, want packages/api=backend", got)
	}
}

func TestEnvironmentOverridesConfigFile(t *testing.T) {
	restore := envSnapshot(t)
	defer restore()

	tmpDir := t.TempDir()
	taskLedgerDir := filepath.Join(tmpDir, ".task-ledger")
	if err := os.MkdirAll(taskLedgerDir, 0750); err != nil {
		t.Fatalf("mkdir .task-ledger: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskLedgerDir, "config.yaml"), []byte("json: false\n"), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Chdir(tmpDir)

	_ = os.Setenv("TL_JSON", "true")
	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() returned error: %v", err)
	}
	if got := GetBool("json"); got != true {
		t.Fatalf("json = %v, want true from env override", got)
	}
}

func TestSetAndNilSafety(t *testing.T) {
	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() returned error: %v", err)
	}

	Set("test-key", "test-value")
	if got := GetString("test-key"); got != "test-value" {
		t.Fatalf("GetString(test-key) = %q, want test-value", got)
	}
	Set("test-int", 42)
	if got := GetInt("test-int"); got != 42 {
		t.Fatalf("GetInt(test-int) = %d, want 42", got)
	}

	saved := v
	v = nil
	defer func() { v = saved }()

	if GetString("any") != "" || GetBool("any") || GetInt("any") != 0 {
		t.Fatalf("nil viper getters returned non-zero values")
	}
	if labels := GetStringMapString("directory.labels"); len(labels) != 0 {
		t.Fatalf("nil viper map = %#v, want empty", labels)
	}
	Set("any", "value")
}

func TestGetDirectoryLabels(t *testing.T) {
	restore := envSnapshot(t)
	defer restore()

	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "packages", "api")
	if err := os.MkdirAll(filepath.Join(workDir, ".task-ledger"), 0750); err != nil {
		t.Fatalf("mkdir .task-ledger: %v", err)
	}
	configPath := filepath.Join(workDir, ".task-ledger", "config.yaml")
	configContent := `
directory:
  labels:
    packages/api: backend
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Chdir(workDir)

	if err := Initialize(); err != nil {
		t.Fatalf("Initialize() returned error: %v", err)
	}
	labels := GetDirectoryLabels()
	if len(labels) != 1 || labels[0] != "backend" {
		t.Fatalf("GetDirectoryLabels() = %#v, want [backend]", labels)
	}
}
