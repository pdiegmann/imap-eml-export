package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, 993, cfg.Port)
	assert.Equal(t, "./output", cfg.OutputDir)
	assert.True(t, cfg.TLS)
}

func TestValidate(t *testing.T) {
	cfg := &Config{
		Host:      "imap.example.com",
		Port:      993,
		Username:  "user@example.com",
		Password:  "secret",
		OutputDir: "./output",
	}
	assert.NoError(t, cfg.Validate())
}

func TestValidateMissingHost(t *testing.T) {
	cfg := &Config{Port: 993, Username: "u", Password: "p", OutputDir: "./o"}
	assert.Error(t, cfg.Validate())
}

func TestValidateMissingUsername(t *testing.T) {
	cfg := &Config{Host: "h", Port: 993, Password: "p", OutputDir: "./o"}
	assert.Error(t, cfg.Validate())
}

func TestValidateInvalidPort(t *testing.T) {
	cfg := &Config{Host: "h", Port: 0, Username: "u", Password: "p", OutputDir: "./o"}
	assert.Error(t, cfg.Validate())
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	original := &Config{
		Host:      "mail.example.com",
		Port:      993,
		Username:  "test@example.com",
		Password:  "mypassword",
		OutputDir: "/tmp/export",
		TLS:       true,
	}

	require.NoError(t, original.Save(path))
	assert.FileExists(t, path)

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, original.Host, loaded.Host)
	assert.Equal(t, original.Port, loaded.Port)
	assert.Equal(t, original.Username, loaded.Username)
	assert.Equal(t, original.OutputDir, loaded.OutputDir)
	assert.Equal(t, original.TLS, loaded.TLS)
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("IMAP_HOST", "env-host.example.com")
	t.Setenv("IMAP_PORT", "143")
	defer os.Unsetenv("IMAP_HOST")
	defer os.Unsetenv("IMAP_PORT")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "env-host.example.com", cfg.Host)
	assert.Equal(t, 143, cfg.Port)
}
