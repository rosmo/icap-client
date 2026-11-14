package icapclient

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestClient_Do(t *testing.T) {
	if !testServerRunning() {
		go startTestServer()
	}

	t.Parallel()

	t.Run("RESPMOD", func(t *testing.T) {
		httpReq, err := http.NewRequest(http.MethodGet, "http://someurl.com", nil)
		if err != nil {
			t.Error(err)
			return
		}

		type testSample struct {
			httpResp         *http.Response
			wantedStatusCode int
			wantedStatus     string
		}

		sampleTable := []testSample{
			{
				httpResp: &http.Response{
					Status:     "200 OK",
					StatusCode: http.StatusOK,
					Proto:      "HTTP/1.0",
					ProtoMajor: 1,
					ProtoMinor: 0,
					Header: http.Header{
						"Content-Type":   []string{"plain/text"},
						"Content-Length": []string{"19"},
					},
					ContentLength: 19,
					Body:          io.NopCloser(strings.NewReader("This is a GOOD FILE")),
				},
				wantedStatusCode: http.StatusNoContent,
				wantedStatus:     "No Modifications",
			},
			{
				httpResp: &http.Response{
					Status:     "200 OK",
					StatusCode: http.StatusOK,
					Proto:      "HTTP/1.0",
					ProtoMajor: 1,
					ProtoMinor: 0,
					Header: http.Header{
						"Content-Type":   []string{"plain/text"},
						"Content-Length": []string{"18"},
					},
					ContentLength: 18,
					Body:          io.NopCloser(strings.NewReader("This is a BAD FILE")),
				},
				wantedStatusCode: http.StatusOK,
				wantedStatus:     "OK",
			},
		}

		for _, sample := range sampleTable {
			req, err := NewRequest(t.Context(), MethodRESPMOD, fmt.Sprintf("icap://localhost:%d/respmod", port), httpReq, sample.httpResp)
			if err != nil {
				t.Error(err)
				return
			}

			client, _ := NewClient()
			resp, err := client.Do(req)
			if err != nil {
				t.Error(err)
				return
			}

			if resp.StatusCode != sample.wantedStatusCode {
				t.Errorf("Wanted status code:%d, got:%d", sample.wantedStatusCode, resp.StatusCode)
			}

			if resp.Status != sample.wantedStatus {
				t.Errorf("Wanted status:%s, got:%s", sample.wantedStatus, resp.Status)
			}
		}
	})

	t.Run("REQMOD", func(t *testing.T) {
		type testSample struct {
			urlStr           string
			wantedStatusCode int
			wantedStatus     string
		}

		sampleTable := []testSample{
			{
				urlStr:           "http://goodifle.com",
				wantedStatusCode: http.StatusNoContent,
				wantedStatus:     "No Modifications",
			},
			{
				urlStr:           "http://badfile.com",
				wantedStatusCode: http.StatusOK,
				wantedStatus:     "OK",
			},
		}

		for _, sample := range sampleTable {
			httpReq, err := http.NewRequest(http.MethodGet, sample.urlStr, nil)
			if err != nil {
				t.Error(err)
				return
			}

			req, err := NewRequest(t.Context(), MethodREQMOD, fmt.Sprintf("icap://localhost:%d/reqmod", port), httpReq, nil)
			if err != nil {
				t.Error(err)
				return
			}

			client, _ := NewClient()
			resp, err := client.Do(req)
			if err != nil {
				t.Error(err)
				return
			}

			if resp.StatusCode != sample.wantedStatusCode {
				t.Errorf("Wanted status code:%d, got:%d", sample.wantedStatusCode, resp.StatusCode)
			}

			if resp.Status != sample.wantedStatus {
				t.Errorf("Wanted status:%s, got:%s", sample.wantedStatus, resp.Status)
			}
		}
	})

	t.Run("RESPMOD with OPTIONS", func(t *testing.T) {
		httpReq, err := http.NewRequest(http.MethodGet, "http://someurl.com", nil)
		if err != nil {
			t.Error(err)
			return
		}

		type testSample struct {
			httpResp               *http.Response
			wantedStatusCode       int
			wantedStatus           string
			wantedPreviewBytes     int
			wantedOptionStatusCode int
			wantedOptionStatus     string
			wantedOptionHeader     http.Header
		}

		sampleTable := []testSample{
			{
				httpResp: &http.Response{
					Status:     "200 OK",
					StatusCode: http.StatusOK,
					Proto:      "HTTP/1.0",
					ProtoMajor: 1,
					ProtoMinor: 0,
					Header: http.Header{
						"Content-Type":   []string{"plain/text"},
						"Content-Length": []string{"41"},
					},
					ContentLength: 41,
					Body:          io.NopCloser(strings.NewReader("Hello World!This is a GOOD FILE! bye bye!")),
				},
				wantedStatusCode:       http.StatusNoContent,
				wantedStatus:           "No Modifications",
				wantedPreviewBytes:     previewBytes,
				wantedOptionStatusCode: http.StatusOK,
				wantedOptionStatus:     "OK",
				wantedOptionHeader: http.Header{
					"Methods":          []string{"RESPMOD"},
					"Allow":            []string{"204"},
					"Preview":          []string{strconv.Itoa(previewBytes)},
					"Transfer-Preview": []string{"*"},
				},
			},
		}

		for _, sample := range sampleTable {
			urlStr := fmt.Sprintf("icap://localhost:%d/respmod", port)

			optReq, err := NewRequest(t.Context(), MethodOPTIONS, urlStr, nil, nil)
			if err != nil {
				t.Error(err)
				return
			}

			client, _ := NewClient()
			optResp, err := client.Do(optReq)
			if err != nil {
				t.Error(err)
				return
			}

			if optResp.Status != sample.wantedOptionStatus {
				t.Errorf("Wanted status:%s, got:%s", sample.wantedOptionStatus, optResp.Status)
			}

			if optResp.StatusCode != sample.wantedOptionStatusCode {
				t.Errorf("Wanted status code:%d, got:%d", sample.wantedOptionStatusCode, optResp.StatusCode)
			}

			if optResp.PreviewBytes != sample.wantedPreviewBytes {
				t.Errorf("Wanted preview bytes:%d , got:%d", sample.wantedPreviewBytes, optResp.PreviewBytes)
			}

			for k, v := range sample.wantedOptionHeader {
				if val, exists := optResp.Header[k]; exists {
					if !reflect.DeepEqual(val, v) {
						t.Errorf("Wanted value for header:%s to be:%v, got:%v", k, v, val)
					}
					continue
				}

				t.Errorf("Expected header:%s but not found", k)
			}

			req, err := NewRequest(t.Context(), MethodRESPMOD, urlStr, httpReq, sample.httpResp)
			if err != nil {
				t.Error(err)
				return
			}

			if err := req.extendHeader(optResp.Header); err != nil {
				t.Error(err)
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Error(err)
				return
			}

			if resp.StatusCode != sample.wantedStatusCode {
				t.Errorf("Wanted status code:%d, got:%d", sample.wantedStatusCode, resp.StatusCode)
			}

			if resp.Status != sample.wantedStatus {
				t.Errorf("Wanted status:%s, got:%s", sample.wantedStatus, resp.Status)
			}
		}
	})

	t.Run("REQMOD with OPTIONS", func(t *testing.T) {
		type testSample struct {
			urlStr                 string
			wantedStatusCode       int
			wantedStatus           string
			wantedOptionStatus     string
			wantedOptionStatusCode int
			wantedOptionHeader     http.Header
		}

		sampleTable := []testSample{
			{
				urlStr:                 "http://goodifle.com",
				wantedStatusCode:       http.StatusNoContent,
				wantedStatus:           "No Modifications",
				wantedOptionStatus:     "OK",
				wantedOptionStatusCode: http.StatusOK,
				wantedOptionHeader: http.Header{
					"Methods":          []string{"REQMOD"},
					"Allow":            []string{"204"},
					"Preview":          []string{strconv.Itoa(previewBytes)},
					"Transfer-Preview": []string{"*"},
				},
			},
			{
				urlStr:                 "http://badfile.com",
				wantedStatusCode:       http.StatusOK,
				wantedStatus:           "OK",
				wantedOptionStatus:     "OK",
				wantedOptionStatusCode: http.StatusOK,
				wantedOptionHeader: http.Header{
					"Methods":          []string{"REQMOD"},
					"Allow":            []string{"204"},
					"Preview":          []string{strconv.Itoa(previewBytes)},
					"Transfer-Preview": []string{"*"},
				},
			},
		}

		for _, sample := range sampleTable {
			urlStr := fmt.Sprintf("icap://localhost:%d/reqmod", port)

			optReq, err := NewRequest(t.Context(), MethodOPTIONS, urlStr, nil, nil)
			if err != nil {
				t.Error(err)
				return
			}

			client, _ := NewClient()
			optResp, err := client.Do(optReq)
			if err != nil {
				t.Error(err)
				return
			}

			if optResp.Status != sample.wantedOptionStatus {
				t.Errorf("Wanted status:%s , got:%s", sample.wantedOptionStatus, optResp.Status)
			}
			if optResp.StatusCode != sample.wantedOptionStatusCode {
				t.Errorf("Wanted status code:%d , got:%d", sample.wantedOptionStatusCode, optResp.StatusCode)
			}
			for k, v := range sample.wantedOptionHeader {
				if val, exists := optResp.Header[k]; exists {
					if !reflect.DeepEqual(val, v) {
						t.Errorf("Wanted header:%s to have value:%v, got:%v", k, v, val)
					}
					continue
				}

				t.Errorf("Expected header:%s but not found", k)
			}

			httpReq, err := http.NewRequest(http.MethodGet, sample.urlStr, nil)
			if err != nil {
				t.Error(err)
				return
			}

			req, err := NewRequest(t.Context(), MethodREQMOD, urlStr, httpReq, nil)
			if err != nil {
				t.Error(err)
				return
			}

			if err := req.extendHeader(optResp.Header); err != nil {
				t.Error(err)
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Error(err)
				return
			}

			if resp.StatusCode != sample.wantedStatusCode {
				t.Errorf("Wanted status code:%d, got:%d", sample.wantedStatusCode, resp.StatusCode)
			}

			if resp.Status != sample.wantedStatus {
				t.Errorf("Wanted status:%s, got:%s", sample.wantedStatus, resp.Status)
			}
		}
	})

	t.Run("Client Do REQMOD with Custom Driver", func(t *testing.T) {
		type testSample struct {
			urlStr           string
			wantedStatusCode int
			wantedStatus     string
		}

		sampleTable := []testSample{
			{
				urlStr:           "http://goodifle.com",
				wantedStatusCode: http.StatusNoContent,
				wantedStatus:     "No Modifications",
			},
			{
				urlStr:           "http://badfile.com",
				wantedStatusCode: http.StatusOK,
				wantedStatus:     "OK",
			},
		}

		for _, sample := range sampleTable {
			httpReq, err := http.NewRequest(http.MethodGet, sample.urlStr, nil)
			if err != nil {
				t.Error(err)
				return
			}

			req, err := NewRequest(t.Context(), MethodREQMOD, fmt.Sprintf("icap://localhost:%d/reqmod", port), httpReq, nil)
			if err != nil {
				t.Error(err)
				return
			}

			client, _ := NewClient()
			resp, err := client.Do(req)
			if err != nil {
				t.Error(err)
				return
			}

			if resp.StatusCode != sample.wantedStatusCode {
				t.Errorf("Wanted status code:%d, got:%d", sample.wantedStatusCode, resp.StatusCode)
			}

			if resp.Status != sample.wantedStatus {
				t.Errorf("Wanted status:%s, got:%s", sample.wantedStatus, resp.Status)
			}
		}
	})

	t.Run("Client Do over H2C Tunnel", func(t *testing.T) {
		h2cLn, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer h2cLn.Close()

		protocols := new(http.Protocols)
		protocols.SetUnencryptedHTTP2(true)

		h2cServer := &http.Server{
			Protocols:         protocols,
			ReadHeaderTimeout: 5 * time.Second,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodConnect || r.ProtoMajor != 2 {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}
				if r.Header.Get("X-Proxy-Token") != "valid-token" {
					http.Error(w, "missing header", http.StatusForbidden)
					return
				}

				backend, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadGateway)
					return
				}
				defer backend.Close()

				rc := http.NewResponseController(w)
				w.WriteHeader(http.StatusOK)
				_ = rc.Flush()

				done := make(chan struct{})
				go func() {
					defer close(done)
					if _, copyErr := io.Copy(backend, r.Body); copyErr != nil {
						t.Errorf("unexpected error copying from client to target: %v", copyErr)
					}
					_ = backend.Close()
				}()

				buf := make([]byte, 4096)
				for {
					n, readErr := backend.Read(buf)
					if n > 0 {
						_, _ = w.Write(buf[:n])
						_ = rc.Flush()
					}
					if readErr != nil {
						break
					}
				}
				<-done
			}),
		}
		go func() {
			_ = h2cServer.Serve(h2cLn)
		}()
		defer h2cServer.Close()

		customTransport := &http.Transport{
			Protocols: protocols,
		}
		defer customTransport.CloseIdleConnections()

		connectHdr := http.Header{
			"X-Proxy-Token": []string{"valid-token"},
		}

		clients := []struct {
			name string
			opts []ConfigOption
		}{
			{
				name: "WithH2CTunnel",
				opts: []ConfigOption{
					WithH2CTunnel(),
					WithH2CConnectHeader(connectHdr),
					WithICAPConnectionTimeout(5 * time.Second),
				},
			},
			{
				name: "WithH2CTransport",
				opts: []ConfigOption{
					WithH2CTransport(customTransport),
					WithH2CConnectHeader(connectHdr),
				},
			},
			{
				name: "WithDialContext",
				opts: []ConfigOption{
					WithDialContext(NewH2CDialContextWithHeader(nil, connectHdr)),
				},
			},
		}

		for _, tc := range clients {
			t.Run(tc.name, func(t *testing.T) {
				urlStr := fmt.Sprintf("icap://%s/respmod", h2cLn.Addr().String())

				optReq, err := NewRequest(t.Context(), MethodOPTIONS, urlStr, nil, nil)
				if err != nil {
					t.Fatal(err)
				}

				client, err := NewClient(tc.opts...)
				if err != nil {
					t.Fatal(err)
				}

				optResp, err := client.Do(optReq)
				if err != nil {
					t.Fatal(err)
				}

				if optResp.StatusCode != http.StatusOK {
					t.Fatalf("Wanted OPTIONS status code:%d, got:%d", http.StatusOK, optResp.StatusCode)
				}

				httpReq, err := http.NewRequest(http.MethodGet, "http://someurl.com", nil)
				if err != nil {
					t.Fatal(err)
				}

				httpResp := &http.Response{
					Status:     "200 OK",
					StatusCode: http.StatusOK,
					Proto:      "HTTP/1.0",
					ProtoMajor: 1,
					ProtoMinor: 0,
					Header: http.Header{
						"Content-Type":   []string{"plain/text"},
						"Content-Length": []string{"41"},
					},
					ContentLength: 41,
					Body:          io.NopCloser(strings.NewReader("Hello World!This is a GOOD FILE! bye bye!")),
				}

				req, err := NewRequest(t.Context(), MethodRESPMOD, urlStr, httpReq, httpResp)
				if err != nil {
					t.Fatal(err)
				}

				if err := req.extendHeader(optResp.Header); err != nil {
					t.Fatal(err)
				}

				resp, err := client.Do(req)
				if err != nil {
					t.Fatal(err)
				}

				if resp.StatusCode != http.StatusNoContent {
					t.Errorf("Wanted status code:%d, got:%d", http.StatusNoContent, resp.StatusCode)
				}
			})
		}
	})

	if testServerRunning() {
		defer stopTestServer()
	}
}
