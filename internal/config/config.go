package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/viper"
)

// GoogleOAuth2Config holds Google OAuth2 credentials used for IMAP OAUTHBEARER
// authentication. ClientID and ClientSecret are optional when provided via
// environment variables GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET.
type GoogleOAuth2Config struct {
	ClientID     string `mapstructure:"client_id"     toml:"client_id,omitempty"`
	ClientSecret string `mapstructure:"client_secret" toml:"client_secret,omitempty"`
	// RefreshToken is stored here after the first successful OAuth2 flow.
	RefreshToken string `mapstructure:"refresh_token" toml:"refresh_token,omitempty"`
}

// ExportConfig holds configuration for the export command.
type ExportConfig struct {
	Host      string             `mapstructure:"host"       toml:"host,omitempty"`
	Port      int                `mapstructure:"port"       toml:"port,omitempty"`
	Username  string             `mapstructure:"username"   toml:"username,omitempty"`
	Password  string             `mapstructure:"password"   toml:"password,omitempty"`
	OutputDir string             `mapstructure:"output_dir" toml:"output_dir,omitempty"`
	TLS       bool               `mapstructure:"tls"        toml:"tls"`
	StartTLS  bool               `mapstructure:"starttls"   toml:"starttls,omitempty"`
	// Google, when true, automatically sets Host/Port/TLS for Gmail/GSuite and
	// uses OAuth2 OAUTHBEARER authentication instead of plain password login.
	Google bool               `mapstructure:"google"     toml:"google,omitempty"`
	OAuth2 GoogleOAuth2Config `mapstructure:"oauth2"     toml:"oauth2,omitempty"`
}

// ImportConfig holds configuration for the import command.
type ImportConfig struct {
	Host     string             `mapstructure:"host"      toml:"host,omitempty"`
	Port     int                `mapstructure:"port"      toml:"port,omitempty"`
	Username string             `mapstructure:"username"  toml:"username,omitempty"`
	Password string             `mapstructure:"password"  toml:"password,omitempty"`
	InputDir string             `mapstructure:"input_dir" toml:"input_dir,omitempty"`
	TLS      bool               `mapstructure:"tls"       toml:"tls"`
	StartTLS bool               `mapstructure:"starttls"  toml:"starttls,omitempty"`
	// Google, when true, automatically sets Host/Port/TLS for Gmail/GSuite and
	// uses OAuth2 OAUTHBEARER authentication instead of plain password login.
	Google bool               `mapstructure:"google"    toml:"google,omitempty"`
	OAuth2 GoogleOAuth2Config `mapstructure:"oauth2"    toml:"oauth2,omitempty"`
}

// Config holds all application settings.
type Config struct {
	Export     ExportConfig `mapstructure:"export" toml:"export"`
	Import     ImportConfig `mapstructure:"import" toml:"import"`
	ConfigFile string       `mapstructure:"-"      toml:"-"`
}

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "imap-eml-export", "config.toml"), nil
}

// Load reads configuration from file, env vars, and applies overrides.
// Priority: CLI flags > env vars > config file > defaults.
//
// Environment variables use the prefix IMAP_ followed by the section and key,
// e.g. IMAP_EXPORT_HOST, IMAP_EXPORT_PORT, IMAP_IMPORT_HOST, etc.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Defaults for export.
	v.SetDefault("export.port", 993)
	v.SetDefault("export.output_dir", "./output")
	v.SetDefault("export.tls", true)
	v.SetDefault("export.starttls", false)

	// Defaults for import.
	v.SetDefault("import.port", 993)
	v.SetDefault("import.tls", true)
	v.SetDefault("import.starttls", false)

	// Env vars: IMAP_EXPORT_HOST → export.host, IMAP_IMPORT_HOST → import.host, …
	v.SetEnvPrefix("IMAP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	for _, key := range []string{
		"export.host", "export.port", "export.username", "export.password",
		"export.output_dir", "export.tls", "export.starttls", "export.google",
		"export.oauth2.client_id", "export.oauth2.client_secret", "export.oauth2.refresh_token",
		"import.host", "import.port", "import.username", "import.password",
		"import.input_dir", "import.tls", "import.starttls", "import.google",
		"import.oauth2.client_id", "import.oauth2.client_secret", "import.oauth2.refresh_token",
	} {
		_ = v.BindEnv(key)
	}

	// Config file.
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		if p, err := DefaultConfigPath(); err == nil {
			v.SetConfigFile(p)
		}
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			if configPath != "" {
				if !errors.Is(err, os.ErrNotExist) {
					return nil, fmt.Errorf("reading config file: %w", err)
				}
			}
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	cfg.ConfigFile = v.ConfigFileUsed()

	return cfg, nil
}

// Save writes the configuration to the given path as TOML.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()
	enc := toml.NewEncoder(f)
	return enc.Encode(c)
}

// ValidateExport checks that all required fields for the export command are set.
func (c *Config) ValidateExport() error {
	ex := &c.Export
	if !ex.Google {
		if ex.Host == "" {
			return errors.New("export.host is required")
		}
	}
	if ex.Port <= 0 || ex.Port > 65535 {
		return fmt.Errorf("invalid export.port: %d", ex.Port)
	}
	if ex.Username == "" {
		return errors.New("export.username is required")
	}
	if !ex.Google && ex.Password == "" {
		return errors.New("export.password is required (or set export.google = true for OAuth2)")
	}
	if ex.OutputDir == "" {
		return errors.New("export.output_dir is required")
	}
	return nil
}

// ValidateImport checks that all required fields for the import command are set.
func (c *Config) ValidateImport() error {
	im := &c.Import
	if !im.Google {
		if im.Host == "" {
			return errors.New("import.host is required")
		}
	}
	if im.Port <= 0 || im.Port > 65535 {
		return fmt.Errorf("invalid import.port: %d", im.Port)
	}
	if im.Username == "" {
		return errors.New("import.username is required")
	}
	if !im.Google && im.Password == "" {
		return errors.New("import.password is required (or set import.google = true for OAuth2)")
	}
	return nil
}

// Validate is kept for backward compatibility and validates only the export config.
// Deprecated: Use ValidateExport or ValidateImport instead.
// Note: this only validates the export configuration, not import configuration.
func (c *Config) Validate() error {
	return c.ValidateExport()
}
