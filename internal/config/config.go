package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure, parsed from a YAML file.
type Config struct {
	// ListenAddr is the address the HTTP server listens on, e.g. ":8080"
	ListenAddr string `yaml:"listen_addr"`

	// VespaURL is the base URL of the Vespa Cloud endpoint,
	// e.g. "https://my-app.vespa-app.cloud"
	VespaURL string `yaml:"vespa_url"`

	// TLS holds mTLS credentials for authenticating against Vespa Cloud.
	TLS TLSConfig `yaml:"tls"`

	// UpstreamTimeoutSec is the timeout for upstream requests in seconds.
	UpstreamTimeoutSec int `yaml:"upstream_timeout_sec"`
}

// TLSConfig holds all TLS / mTLS options.
// File paths and inline PEM values are mutually exclusive per field;
// inline PEM takes precedence when both are set.
type TLSConfig struct {
	// File paths (mounted secrets, ConfigMaps, etc.)
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	CAFile   string `yaml:"ca_file"` // optional; for server certificate verification

	// Inline PEM (useful for passing secrets via env-substituted YAML or CI)
	CertPEM string `yaml:"cert_pem"`
	KeyPEM  string `yaml:"key_pem"`
	CAPEM   string `yaml:"ca_pem"` // optional

	// SkipVerify disables server certificate verification. Dev only!
	SkipVerify bool `yaml:"skip_verify"`
}

// Load reads and validates configuration from the YAML file at configPath.
// If configPath is empty, it falls back to the CONFIG_FILE environment
// variable, and finally to "config.yaml" in the working directory.
func Load(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = envOrDefault("CONFIG_FILE", "config.yaml")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %q: %w", configPath, err)
	}

	// Allow ${VAR} / $VAR substitution in the YAML so that secrets can be
	// injected via environment variables without a separate secrets manager.
	expanded := os.ExpandEnv(string(data))

	cfg := &Config{
		ListenAddr:         ":8080",
		UpstreamTimeoutSec: 30,
	}
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", configPath, err)
	}

	return cfg, validate(cfg)
}

func validate(cfg *Config) error {
	var errs []string

	if cfg.VespaURL == "" {
		errs = append(errs, "vespa_url is required")
	}
	cfg.VespaURL = strings.TrimRight(cfg.VespaURL, "/")

	if cfg.UpstreamTimeoutSec <= 0 {
		errs = append(errs, "upstream_timeout_sec must be a positive integer")
	}

	hasCert := cfg.TLS.CertFile != "" || cfg.TLS.CertPEM != ""
	hasKey := cfg.TLS.KeyFile != "" || cfg.TLS.KeyPEM != ""

	if hasCert && !hasKey {
		errs = append(errs, "tls.key_file or tls.key_pem is required when a certificate is provided")
	}
	if !hasCert && hasKey {
		errs = append(errs, "tls.cert_file or tls.cert_pem is required when a key is provided")
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
