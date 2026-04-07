package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// Proxy is the reverse proxy that intercepts Anthropic API requests.
type Proxy struct {
	target  *url.URL
	webAddr string
	rp      *httputil.ReverseProxy
}

// NewProxy creates a new proxy targeting the given upstream URL.
// webAddr is the base URL of the web server for pushing composition data.
func NewProxy(targetURL, webAddr string) *Proxy {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalf("invalid target URL: %v", err)
	}

	p := &Proxy{
		target:  target,
		webAddr: webAddr,
	}

	p.rp = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
	}

	return p
}

// Handler returns the HTTP handler for the proxy.
func (p *Proxy) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only intercept POST /v1/messages
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/v1/messages") {
			p.rp.ServeHTTP(w, r)
			return
		}

		// Capture request body
		reqBody, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadGateway)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(reqBody))
		r.ContentLength = int64(len(reqBody))

		// Parse the request body
		parsed, parseErr := ParseRequestBody(reqBody)

		// Use a response recorder to capture the response
		rec := &responseCapture{ResponseWriter: w}
		p.rp.ServeHTTP(rec, r)

		// Process composition in background (don't block the response)
		if parseErr == nil && rec.statusCode >= 200 && rec.statusCode < 300 {
			// Extract session ID from x-session-id header if present, otherwise empty
			sessionID := r.Header.Get("x-session-id")
			go p.processComposition(parsed, rec.body.Bytes(), sessionID)
		}
	})
}

func (p *Proxy) processComposition(parsed ParseResult, respBody []byte, sessionID string) {
	// Extract usage.input_tokens from response
	var respData struct {
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &respData); err != nil {
		log.Printf("proxy: parse response usage: %v", err)
		return
	}

	ctxSize := LookupContextWindowSize(parsed.Model, parsed.HasExtendedThinking)
	snap := Estimate(parsed, respData.Usage.InputTokens, ctxSize)
	snap.SessionID = sessionID

	if err := PushComposition(p.webAddr, snap); err != nil {
		log.Printf("proxy: push composition: %v", err)
	}
}

// responseCapture wraps ResponseWriter to capture the response body and status.
type responseCapture struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.statusCode = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}
