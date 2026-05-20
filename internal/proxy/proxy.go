package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"

	"vespa-proxy/internal/config"
)

// New creates an http.Handler that reverse-proxies requests to Vespa Cloud
// with mTLS configured from cfg.
func New(cfg *config.Config, logger *slog.Logger) (http.Handler, error) {
	target, err := url.Parse(cfg.VespaURL)
	if err != nil {
		return nil, fmt.Errorf("invalid VESPA_URL %q: %w", cfg.VespaURL, err)
	}

	tlsCfg, err := buildTLSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building TLS config: %w", err)
	}

	transport := &http.Transport{
		TLSClientConfig:       tlsCfg,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   20,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: time.Duration(cfg.UpstreamTimeoutSec) * time.Second,
	}

	rp := &httputil.ReverseProxy{
		Rewrite:   rewrite(target),
		Transport: transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logger.Error("proxy error",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
		ModifyResponse: func(resp *http.Response) error {
			// Remove hop-by-hop headers that should not be forwarded
			resp.Header.Del("Alt-Svc")
			return nil
		},
	}

	return rp, nil
}

// rewrite rewrites the incoming request URL to point at the upstream target.
func rewrite(target *url.URL) func(*httputil.ProxyRequest) {
	return func(pr *httputil.ProxyRequest) {
		pr.SetURL(target)

		// Preserve existing path; target path acts as prefix when non-empty
		if target.Path != "" && target.Path != "/" {
			pr.Out.URL.Path = target.Path + pr.In.URL.Path
		}

		// Forward the real client IP
		pr.SetXForwarded()
		pr.Out.Header.Set("X-Forwarded-Proto", "https")

		// Vespa Cloud requires the SNI hostname
		pr.Out.Host = target.Host
	}
}

// buildTLSConfig assembles a *tls.Config from the configuration.
// Certificate material can come from files (TLS_CERT_FILE / TLS_KEY_FILE)
// or from inline PEM environment variables (TLS_CERT_PEM / TLS_KEY_PEM).
func buildTLSConfig(cfg *config.Config) (*tls.Config, error) {
	tlsCfg := &tls.Config{
		InsecureSkipVerify: cfg.TLS.SkipVerify, //nolint:gosec // intentional opt-in
		MinVersion:         tls.VersionTLS12,
	}

	// ── Client certificate (mTLS) ─────────────────────────────────────────
	certPEM, keyPEM, err := resolvePEMPair(
		cfg.TLS.CertPEM, cfg.TLS.CertFile,
		cfg.TLS.KeyPEM, cfg.TLS.KeyFile,
	)
	if err != nil {
		return nil, err
	}

	if len(certPEM) > 0 {
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("parsing client certificate/key: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	// ── CA bundle for server verification ────────────────────────────────
	caPEM, err := resolvePEM(cfg.TLS.CAPEM, cfg.TLS.CAFile)
	if err != nil {
		return nil, err
	}
	if len(caPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("no valid CA certificates found in TLS_CA_FILE/TLS_CA_PEM")
		}
		tlsCfg.RootCAs = pool
	}

	return tlsCfg, nil
}

// resolvePEMPair returns PEM bytes for cert and key, preferring inline values.
func resolvePEMPair(certPEM, certFile, keyPEM, keyFile string) ([]byte, []byte, error) {
	cert, err := resolvePEM(certPEM, certFile)
	if err != nil {
		return nil, nil, fmt.Errorf("certificate: %w", err)
	}
	key, err := resolvePEM(keyPEM, keyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("private key: %w", err)
	}
	return cert, key, nil
}

// resolvePEM returns inline PEM if non-empty, otherwise reads the file.
func resolvePEM(inline, filePath string) ([]byte, error) {
	if inline != "" {
		return []byte(inline), nil
	}
	if filePath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
	}
	return data, nil
}
