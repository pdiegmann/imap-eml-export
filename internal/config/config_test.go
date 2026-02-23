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
	assert.Equal(t, 993, cfg.Export.Port)
	assert.Equal(t, "./output", cfg.Export.OutputDir)
	assert.True(t, cfg.Export.TLS)
	assert.Equal(t, 993, cfg.Import.Port)
	assert.True(t, cfg.Import.TLS)
}

func TestValidateExport(t *testing.T) {
	cfg := &Config{
		Export: ExportConfig{
			Host:      "imap.example.com",
			Port:      993,
			Username:  "user@example.com",
			Password:  "secret",
			OutputDir: "./output",
		},
	}
	assert.NoError(t, cfg.ValidateExport())
}

func TestValidateImport(t *testing.T) {
	cfg := &Config{
		Import: ImportConfig{
			Host:     "imap.example.com",
			Port:     993,
			Username: "user@example.com",
			Password: "secret",
		},
	}
	assert.NoError(t, cfg.ValidateImport())
}

func TestValidateMissingExportHost(t *testing.T) {
	cfg := &Config{Export: ExportConfig{Port: 993, Username: "u", Password: "p", OutputDir: "./o"}}
	assert.Error(t, cfg.ValidateExport())
}

func TestValidateMissingExportUsername(t *testing.T) {
	cfg := &Config{Export: ExportConfig{Host: "h", Port: 993, Password: "p", OutputDir: "./o"}}
	assert.Error(t, cfg.ValidateExport())
}

func TestValidateInvalidExportPort(t *testing.T) {
	cfg := &Config{Export: ExportConfig{Host: "h", Port: 0, Username: "u", Password: "p", OutputDir: "./o"}}
	assert.Error(t, cfg.ValidateExport())
}

func TestValidateGoogleSkipsHostAndPasswordCheck(t *testing.T) {
	cfg := &Config{
		Export: ExportConfig{
			Port:      993,
			Username:  "user@gmail.com",
			OutputDir: "./output",
			Google:    true,
		},
	}
	// Google mode: host and password are not required (host is auto-set, password replaced by OAuth2).
	assert.NoError(t, cfg.ValidateExport())
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	original := &Config{
		Export: ExportConfig{
			Host:      "mail.example.com",
			Port:      993,
			Username:  "test@example.com",
			Password:  "mypassword",
			OutputDir: "/tmp/export",
			TLS:       true,
		},
		Import: ImportConfig{
			Host:     "mail2.example.com",
			Port:     993,
			Username: "test2@example.com",
			Password: "mypassword2",
			TLS:      true,
		},
	}

	require.NoError(t, original.Save(path))
	assert.FileExists(t, path)

	loaded, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, original.Export.Host, loaded.Export.Host)
	assert.Equal(t, original.Export.Port, loaded.Export.Port)
	assert.Equal(t, original.Export.Username, loaded.Export.Username)
	assert.Equal(t, original.Export.OutputDir, loaded.Export.OutputDir)
	assert.Equal(t, original.Export.TLS, loaded.Export.TLS)
	assert.Equal(t, original.Import.Host, loaded.Import.Host)
	assert.Equal(t, original.Import.Username, loaded.Import.Username)
}

func TestEnvVarOverride(t *testing.T) {
	t.Setenv("IMAP_EXPORT_HOST", "env-export-host.example.com")
	t.Setenv("IMAP_EXPORT_PORT", "143")
	t.Setenv("IMAP_IMPORT_HOST", "env-import-host.example.com")
	defer os.Unsetenv("IMAP_EXPORT_HOST")
	defer os.Unsetenv("IMAP_EXPORT_PORT")
	defer os.Unsetenv("IMAP_IMPORT_HOST")

	cfg, err := Load("")
	require.NoError(t, err)
	assert.Equal(t, "env-export-host.example.com", cfg.Export.Host)
	assert.Equal(t, 143, cfg.Export.Port)
	assert.Equal(t, "env-import-host.example.com", cfg.Import.Host)
}
