package icapclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// DialContextFunc defines a function that establishes a network connection.
type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

// NewH2CDialContext returns a DialContextFunc that wraps the connection in an
// HTTP/2 cleartext (h2c) tunnel using the HTTP CONNECT method.
// If rt is nil, a default unencrypted HTTP/2 transport is used.
func NewH2CDialContext(rt http.RoundTripper) DialContextFunc {
	return newH2CDialContext(rt, 0)
}

func newH2CDialContext(rt http.RoundTripper, timeout time.Duration) DialContextFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		roundTripper := rt
		var ownedTransport *http.Transport
		if roundTripper == nil {
			ownedTransport = defaultH2CTransport(timeout)
			roundTripper = ownedTransport
		}

		pr, pw := io.Pipe()

		req, err := http.NewRequestWithContext(ctx, http.MethodConnect, "http://"+address, pr)
		if err != nil {
			_ = pr.Close()
			_ = pw.Close()
			return nil, err
		}
		req.ContentLength = -1

		resp, err := roundTripper.RoundTrip(req)
		if err != nil {
			_ = pr.Close()
			_ = pw.Close()
			if ownedTransport != nil {
				ownedTransport.CloseIdleConnections()
			}
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			_ = pw.Close()
			_ = resp.Body.Close()
			if ownedTransport != nil {
				ownedTransport.CloseIdleConnections()
			}
			return nil, fmt.Errorf("%w: unexpected CONNECT status %s", ErrInvalidConnection, resp.Status)
		}

		addr := h2cAddr{
			network: network,
			address: address,
		}

		return &h2cConn{
			reader:         resp.Body,
			writer:         pw,
			ownedTransport: ownedTransport,
			localAddr:      addr,
			remoteAddr:     addr,
		}, nil
	}
}

func defaultH2CTransport(timeout time.Duration) *http.Transport {
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	return &http.Transport{
		Protocols:   protocols,
		DialContext: dialer.DialContext,
	}
}

type h2cAddr struct {
	network string
	address string
}

func (a h2cAddr) Network() string { return a.network }
func (a h2cAddr) String() string  { return a.address }

type h2cConn struct {
	reader         io.ReadCloser
	writer         *io.PipeWriter
	ownedTransport *http.Transport
	localAddr      net.Addr
	remoteAddr     net.Addr

	mu            sync.Mutex
	readTimer     *time.Timer
	writeTimer    *time.Timer
	readTimedOut  atomic.Bool
	writeTimedOut atomic.Bool
}

func (c *h2cConn) Read(b []byte) (int, error) {
	if c.readTimedOut.Load() {
		return 0, os.ErrDeadlineExceeded
	}

	n, err := c.reader.Read(b)
	if err != nil && c.readTimedOut.Load() {
		return n, os.ErrDeadlineExceeded
	}

	return n, err
}

func (c *h2cConn) Write(b []byte) (int, error) {
	if c.writeTimedOut.Load() {
		return 0, os.ErrDeadlineExceeded
	}

	n, err := c.writer.Write(b)
	if err != nil && c.writeTimedOut.Load() {
		return n, os.ErrDeadlineExceeded
	}

	return n, err
}

func (c *h2cConn) Close() error {
	c.mu.Lock()
	if c.readTimer != nil {
		c.readTimer.Stop()
		c.readTimer = nil
	}
	if c.writeTimer != nil {
		c.writeTimer.Stop()
		c.writeTimer = nil
	}
	c.mu.Unlock()

	err := errors.Join(c.writer.Close(), c.reader.Close())
	if c.ownedTransport != nil {
		c.ownedTransport.CloseIdleConnections()
	}

	return err
}

func (c *h2cConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *h2cConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *h2cConn) SetDeadline(t time.Time) error {
	return errors.Join(c.SetReadDeadline(t), c.SetWriteDeadline(t))
}

func (c *h2cConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.readTimer != nil {
		c.readTimer.Stop()
		c.readTimer = nil
	}
	c.readTimedOut.Store(false)

	if t.IsZero() {
		return nil
	}

	d := time.Until(t)
	if d <= 0 {
		c.readTimedOut.Store(true)
		_ = c.reader.Close()
		return nil
	}

	c.readTimer = time.AfterFunc(d, func() {
		c.readTimedOut.Store(true)
		_ = c.reader.Close()
	})

	return nil
}

func (c *h2cConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.writeTimer != nil {
		c.writeTimer.Stop()
		c.writeTimer = nil
	}
	c.writeTimedOut.Store(false)

	if t.IsZero() {
		return nil
	}

	d := time.Until(t)
	if d <= 0 {
		c.writeTimedOut.Store(true)
		_ = c.writer.CloseWithError(os.ErrDeadlineExceeded)
		return nil
	}

	c.writeTimer = time.AfterFunc(d, func() {
		c.writeTimedOut.Store(true)
		_ = c.writer.CloseWithError(os.ErrDeadlineExceeded)
	})

	return nil
}
