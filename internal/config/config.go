package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/spf13/viper"
)

// Config holds all application settings.
type Config struct {
	Host       string `mapstructure:"host"        toml:"host"`
	Port       int    `mapstructure:"port"        toml:"port"`
	Username   string `mapstructure:"username"    toml:"username"`
	Password   string `mapstructure:"password"    toml:"password"`
	OutputDir  string `mapstructure:"output_dir"  toml:"output_dir"`
	TLS        bool   `mapstructure:"tls"         toml:"tls"`
	StartTLS   bool   `mapstructure:"starttls"    toml:"starttls"`
	ConfigFile string `mapstructure:"-"           toml:"-"`
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
// Priority: CLI flags > env vars > config file > defaults
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Defaults
	v.SetDefault("port", 993)
	v.SetDefault("output_dir", "./output")
	v.SetDefault("tls", true)
	v.SetDefault("starttls", false)

	// Env vars
	v.SetEnvPrefix("IMAP")
	v.AutomaticEnv()
	// Explicitly bind env vars so Unmarshal picks them up in all viper versions.
	for _, key := range []string{"host", "port", "username", "password", "output_dir", "tls", "starttls"} {
		_ = v.BindEnv(key) // BindEnv only fails on empty key, which cannot happen here
	}

	// Config file
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

// Validate checks that all required fields are set.
func (c *Config) Validate() error {
	if c.Host == "" {
		return errors.New("host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.Username == "" {
		return errors.New("username is required")
	}
	if c.Password == "" {
		return errors.New("password is required")
	}
	if c.OutputDir == "" {
		return errors.New("output_dir is required")
	}
	return nil
}
