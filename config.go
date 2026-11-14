package icapclient

import (
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
		cfg.ICAPConn.DialContext = NewH2CDialContext(rt)
	}
}
