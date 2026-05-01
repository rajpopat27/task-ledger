package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"task-ledger/internal/debug"
)

var v *viper.Viper

// Initialize sets up process configuration from .task-ledger/config.yaml, user config,
// and TL_* environment variables.
func Initialize() error {
	v = viper.New()
	v.SetConfigType("yaml")

	configFileSet := false

	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
			configPath := filepath.Join(dir, ".task-ledger", "config.yaml")
			if _, err := os.Stat(configPath); err == nil {
				v.SetConfigFile(configPath)
				configFileSet = true
				break
			}
		}
	}

	if !configFileSet {
		if configDir, err := os.UserConfigDir(); err == nil {
			configPath := filepath.Join(configDir, "tl", "config.yaml")
			if _, err := os.Stat(configPath); err == nil {
				v.SetConfigFile(configPath)
				configFileSet = true
			}
		}
	}

	if !configFileSet {
		if homeDir, err := os.UserHomeDir(); err == nil {
			configPath := filepath.Join(homeDir, ".task-ledger", "config.yaml")
			if _, err := os.Stat(configPath); err == nil {
				v.SetConfigFile(configPath)
				configFileSet = true
			}
		}
	}

	v.SetEnvPrefix("TL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	_ = v.BindEnv("actor", "TASK_LEDGER_ACTOR")

	v.SetDefault("json", false)
	v.SetDefault("readonly", false)
	v.SetDefault("actor", "")
	v.SetDefault("create.require-description", false)
	v.SetDefault("validation.on-create", "none")
	v.SetDefault("hierarchy.max-depth", 3)
	v.SetDefault("directory.labels", map[string]string{})

	if configFileSet {
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("error reading config file: %w", err)
		}
		debug.Logf("Debug: loaded config from %s\n", v.ConfigFileUsed())
	} else {
		debug.Logf("Debug: no config.yaml found; using defaults and environment variables\n")
	}

	return nil
}

// GetString retrieves a string configuration value.
func GetString(key string) string {
	if v == nil {
		return ""
	}
	return v.GetString(key)
}

// GetBool retrieves a boolean configuration value.
func GetBool(key string) bool {
	if v == nil {
		return false
	}
	return v.GetBool(key)
}

// GetInt retrieves an integer configuration value.
func GetInt(key string) int {
	if v == nil {
		return 0
	}
	return v.GetInt(key)
}

// Set overrides a configuration value for the current process.
func Set(key string, value interface{}) {
	if v != nil {
		v.Set(key, value)
	}
}

// GetStringMapString retrieves a map[string]string configuration value.
func GetStringMapString(key string) map[string]string {
	if v == nil {
		return map[string]string{}
	}
	return v.GetStringMapString(key)
}

// GetDirectoryLabels returns labels for the current working directory based on
// directory.labels patterns in config.yaml.
func GetDirectoryLabels() []string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	dirLabels := GetStringMapString("directory.labels")
	if len(dirLabels) == 0 {
		return nil
	}

	for pattern, label := range dirLabels {
		cleanPattern := filepath.Clean(pattern)
		if strings.HasSuffix(cwd, pattern) || strings.HasSuffix(cwd, cleanPattern) {
			return []string{label}
		}
		if strings.Contains(cwd, string(filepath.Separator)+cleanPattern+string(filepath.Separator)) ||
			strings.Contains(cwd, string(filepath.Separator)+cleanPattern) {
			return []string{label}
		}
	}

	return nil
}
