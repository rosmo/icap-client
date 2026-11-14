package icapclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// ICAPConnConfig is the configuration for the icap connection.
type ICAPConnConfig struct {
	// Timeout is the maximum amount of time a connection will be kept open
	Timeout time.Duration
	// DialContext specifies the dial function for creating connections.
	// If DialContext is nil, then the connection dials using net.Dialer (or h2c if H2CTunnel is true).
	DialContext DialContextFunc
	// H2CTunnel wraps the ICAP connection in an HTTP/2 h2c tunnel using CONNECT.
	H2CTunnel bool
	// H2CTransport specifies a custom http.RoundTripper for the HTTP/2 h2c CONNECT tunnel.
	H2CTransport http.RoundTripper
	// H2CConnectHeader specifies custom HTTP headers to include in the HTTP/2 CONNECT request.
	H2CConnectHeader http.Header
}

// ICAPConn manages the transport layer for ICAP protocol.
type ICAPConn struct {
	tcp         net.Conn
	mu          sync.Mutex
	timeout     time.Duration
	dialContext DialContextFunc
}

// NewICAPConn creates a new connection configuration.
func NewICAPConn(conf ICAPConnConfig) (*ICAPConn, error) {
	dialContext := conf.DialContext
	if dialContext == nil && (conf.H2CTunnel || conf.H2CTransport != nil) {
		dialContext = newH2CDialContext(conf.H2CTransport, conf.Timeout, conf.H2CConnectHeader)
	}

	return &ICAPConn{
		timeout:     conf.Timeout,
		dialContext: dialContext,
	}, nil
}

// Connect connects to the ICAP server.
func (c *ICAPConn) Connect(ctx context.Context, address string) error {
	dial := c.dialContext
	if dial == nil {
		dialer := net.Dialer{Timeout: c.timeout}
		dial = dialer.DialContext
	}

	conn, err := dial(ctx, "tcp", address)
	if err != nil {
		return err
	}
	c.tcp = conn

	if c.timeout > 0 {
		deadline := time.Now().Add(c.timeout)
		if err := c.tcp.SetDeadline(deadline); err != nil {
			return err
		}
	}

	return nil
}

// Send sends a request to the ICAP server and reads the response.
func (c *ICAPConn) Send(in []byte) ([]byte, error) {
	if !c.ok() {
		return nil, ErrInvalidConnection
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.tcp.Write(in)
	if err != nil {
		return nil, err
	}

	var data []byte
	buf := make([]byte, 4096)
	for {
		n, err := c.tcp.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}

		if errors.Is(err, io.EOF) || n == 0 {
			break
		}

		data = append(data, buf[:n]...)

		// Protocol checks for message termination
		{
			if bytes.Equal(data, []byte(icap100ContinueMsg)) {
				break
			}

			if bytes.HasSuffix(data, []byte(doubleCRLF)) {
				break
			}

			if bytes.Contains(data, []byte(icap204NoModsMsg)) {
				break
			}
		}
	}

	return data, nil
}

// Close closes the TCP connection.
func (c *ICAPConn) Close() error {
	if !c.ok() {
		return ErrInvalidConnection
	}
	return c.tcp.Close()
}

func (c *ICAPConn) ok() bool {
	return c != nil && c.tcp != nil
}
