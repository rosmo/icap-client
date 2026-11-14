package icapclient

import (
	"crypto/tls"
	"net/http"
	"time"
)

// Config is the shared configuration for the icap client library.
type Config struct {
	ICAPConn ICAPConnConfig
}

// DefaultConfig returns the default configuration for the icap client library.
func DefaultConfig() Config {
	return Config{
		ICAPConn: ICAPConnConfig{
			Timeout: 15 * time.Second,
		},
	}
}

// ConfigOption is a function that configures the icap client.
type ConfigOption func(*Config)

// WithICAPConnectionTimeout sets the timeout for the connection to the icap server.
func WithICAPConnectionTimeout(timeout time.Duration) ConfigOption {
	return func(cfg *Config) {
		if timeout <= 0 {
			return
		}

		cfg.ICAPConn.Timeout = timeout
	}
}

// WithDialContext sets a custom dialer function for connecting to the icap server.
func WithDialContext(dialContext DialContextFunc) ConfigOption {
	return func(cfg *Config) {
		cfg.ICAPConn.DialContext = dialContext
	}
}

// WithH2CTunnel configures the client to wrap the ICAP connection in an HTTP/2 h2c tunnel using CONNECT.
func WithH2CTunnel() ConfigOption {
	return func(cfg *Config) {
		cfg.ICAPConn.H2CTunnel = true
	}
}

// WithH2CTransport configures the client to wrap the ICAP connection in an HTTP/2 h2c tunnel
// using CONNECT over the provided http.RoundTripper.
func WithH2CTransport(rt http.RoundTripper) ConfigOption {
	return func(cfg *Config) {
		cfg.ICAPConn.H2CTunnel = true
		cfg.ICAPConn.H2CTransport = rt
	}
}

// WithH2CConnectHeader sets custom HTTP headers to include in the HTTP/2 CONNECT request.
func WithH2CConnectHeader(header http.Header) ConfigOption {
	return func(cfg *Config) {
		if header == nil {
			return
		}
		cfg.ICAPConn.H2CConnectHeader = header.Clone()
	}
}

// WithH2Tunnel configures the client to wrap the ICAP connection in an encrypted
// HTTP/2 (h2 over TLS) tunnel using CONNECT.
func WithH2Tunnel() ConfigOption {
	return func(cfg *Config) {
		cfg.ICAPConn.H2Tunnel = true
	}
}

// WithTLSConfig sets the TLS configuration used when establishing an encrypted HTTP/2 (h2) tunnel.
func WithTLSConfig(tlsConfig *tls.Config) ConfigOption {
	return func(cfg *Config) {
		if tlsConfig == nil {
			return
		}
		cfg.ICAPConn.TLSConfig = tlsConfig.Clone()
	}
}

// WithH2Transport configures the client to wrap the ICAP connection in an encrypted
// HTTP/2 (h2 over TLS) tunnel using CONNECT over the provided http.RoundTripper.
func WithH2Transport(rt http.RoundTripper) ConfigOption {
	return func(cfg *Config) {
		cfg.ICAPConn.H2Tunnel = true
		cfg.ICAPConn.H2CTransport = rt
	}
}

// WithH2ConnectHeader sets custom HTTP headers to include in the HTTP/2 CONNECT request.
func WithH2ConnectHeader(header http.Header) ConfigOption {
	return WithH2CConnectHeader(header)
}
