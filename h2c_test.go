package icapclient_test

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/phayes/freeport"
	icapclient "github.com/rosmo/icap-client"
)

func startH2CTunnelServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (string, func()) {
	t.Helper()

	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}

	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Handler:           http.HandlerFunc(handler),
		Protocols:         protocols,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		_ = srv.Serve(ln)
	}()

	cleanup := func() {
		_ = srv.Close()
		_ = ln.Close()
	}

	return ln.Addr().String(), cleanup
}

func TestH2CTunnel_ICAPConn_Send(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
		want     string
	}{
		{
			name:     "icap100ContinueMsg",
			response: icapclient.ICAP100ContinueMsg,
			want:     icapclient.ICAP100ContinueMsg,
		},
		{
			name:     "doubleCRLF",
			response: "prefix" + icapclient.DoubleCRLF,
			want:     "prefix" + icapclient.DoubleCRLF,
		},
		{
			name:     "icap204NoModsMsg",
			response: "prefix" + icapclient.ICAP204NoModsMsg + "suffix",
			want:     "prefix" + icapclient.ICAP204NoModsMsg + "suffix",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			serverDone := make(chan error, 1)
			var gotConnectHeader string

			addr, cleanup := startH2CTunnelServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodConnect {
					http.Error(w, "expected CONNECT", http.StatusMethodNotAllowed)
					return
				}
				if r.ProtoMajor != 2 {
					http.Error(w, "expected HTTP/2", http.StatusHTTPVersionNotSupported)
					return
				}
				gotConnectHeader = r.Header.Get("X-Tunnel-Auth")

				rc := http.NewResponseController(w)
				w.WriteHeader(http.StatusOK)
				_ = rc.Flush()

				buf := make([]byte, 1024)
				_, err := r.Body.Read(buf)
				if err != nil && !errors.Is(err, io.EOF) {
					serverDone <- err
					return
				}

				_, _ = w.Write([]byte(tc.response))
				_ = rc.Flush()

				_, copyErr := io.Copy(io.Discard, r.Body)
				serverDone <- copyErr
			})
			defer cleanup()

			clientConn, err := icapclient.NewICAPConn(icapclient.ICAPConnConfig{
				Timeout:   5 * time.Second,
				H2CTunnel: true,
				H2CConnectHeader: http.Header{
					"X-Tunnel-Auth": []string{"secret-token"},
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			if err := clientConn.Connect(t.Context(), addr); err != nil {
				t.Fatal(err)
			}

			res, err := clientConn.Send([]byte("OPTIONS icap://localhost/ ICAP/1.0\r\n\r\n"))
			if err != nil {
				t.Fatal(err)
			}

			if got := string(res); got != tc.want {
				t.Errorf("ICAPConn.Send() = %q, want %q", got, tc.want)
			}

			if gotConnectHeader != "secret-token" {
				t.Errorf("CONNECT header X-Tunnel-Auth = %q, want %q", gotConnectHeader, "secret-token")
			}

			if err := clientConn.Close(); err != nil {
				t.Fatalf("ICAPConn.Close() error = %v", err)
			}

			if copyErr := <-serverDone; copyErr != nil {
				t.Fatalf("unexpected server r.Body error on close: %v", copyErr)
			}
		})
	}
}

func TestH2TLSTunnel_ICAPConn_Send(t *testing.T) {
	t.Parallel()

	serverDone := make(chan error, 1)
	var gotConnectHeader string

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "expected CONNECT", http.StatusMethodNotAllowed)
			return
		}
		if r.ProtoMajor != 2 || r.TLS == nil {
			http.Error(w, "expected HTTP/2 over TLS", http.StatusHTTPVersionNotSupported)
			return
		}
		gotConnectHeader = r.Header.Get("X-Tunnel-Auth")

		rc := http.NewResponseController(w)
		w.WriteHeader(http.StatusOK)
		_ = rc.Flush()

		buf := make([]byte, 1024)
		_, err := r.Body.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			serverDone <- err
			return
		}

		_, _ = w.Write([]byte(icapclient.ICAP204NoModsMsg))
		_ = rc.Flush()

		_, copyErr := io.Copy(io.Discard, r.Body)
		serverDone <- copyErr
	}))
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()

	clientConn, err := icapclient.NewICAPConn(icapclient.ICAPConnConfig{
		Timeout:   5 * time.Second,
		H2Tunnel:  true,
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
		H2CConnectHeader: http.Header{
			"X-Tunnel-Auth": []string{"tls-secret-token"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := clientConn.Connect(t.Context(), srv.Listener.Addr().String()); err != nil {
		t.Fatal(err)
	}

	res, err := clientConn.Send([]byte("OPTIONS icap://localhost/ ICAP/1.0\r\n\r\n"))
	if err != nil {
		t.Fatal(err)
	}

	if got := string(res); got != icapclient.ICAP204NoModsMsg {
		t.Errorf("ICAPConn.Send() = %q, want %q", got, icapclient.ICAP204NoModsMsg)
	}

	if gotConnectHeader != "tls-secret-token" {
		t.Errorf("CONNECT header X-Tunnel-Auth = %q, want %q", gotConnectHeader, "tls-secret-token")
	}

	if err := clientConn.Close(); err != nil {
		t.Fatalf("ICAPConn.Close() error = %v", err)
	}

	if copyErr := <-serverDone; copyErr != nil {
		t.Fatalf("unexpected server r.Body error on close: %v", copyErr)
	}
}

func TestH2CTunnel_StatusError(t *testing.T) {
	t.Parallel()

	addr, cleanup := startH2CTunnelServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	defer cleanup()

	clientConn, err := icapclient.NewICAPConn(icapclient.ICAPConnConfig{
		Timeout:   5 * time.Second,
		H2CTunnel: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = clientConn.Connect(t.Context(), addr)
	if !errors.Is(err, icapclient.ErrInvalidConnection) {
		t.Fatalf("expected ErrInvalidConnection, got %v", err)
	}
}

func TestH2CTunnel_DeadlinesAndAddrs(t *testing.T) {
	t.Parallel()

	addr, cleanup := startH2CTunnelServer(t, func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		w.WriteHeader(http.StatusOK)
		_ = rc.Flush()

		// Hold the stream open until client disconnects.
		_, _ = io.Copy(io.Discard, r.Body)
	})
	defer cleanup()

	dial := icapclient.NewH2CDialContext(nil)
	conn, err := dial(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if conn.LocalAddr().Network() != "tcp" || conn.RemoteAddr().String() != addr {
		t.Errorf("unexpected addrs: local=%v remote=%v", conn.LocalAddr(), conn.RemoteAddr())
	}

	// Past read deadline should immediately trigger ErrDeadlineExceeded.
	if err := conn.SetReadDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 16)
	_, err = conn.Read(buf)
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("expected os.ErrDeadlineExceeded on Read, got %v", err)
	}

	// Past write deadline should trigger ErrDeadlineExceeded.
	if err := conn.SetWriteDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}

	_, err = conn.Write([]byte("ping"))
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("expected os.ErrDeadlineExceeded on Write, got %v", err)
	}

	// Resetting deadline to zero should succeed.
	if err := conn.SetDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
}
