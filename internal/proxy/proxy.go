package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

var proxyLog *log.Logger

func init() {
	f, err := os.OpenFile("/tmp/claude-monitor-proxy.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("proxy: could not open log file: %v", err)
		proxyLog = log.New(io.Discard, "", 0)
		return
	}
	proxyLog = log.New(f, "", 0)
}

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
			// Strip Accept-Encoding so the backend sends plain-text SSE instead
			// of a gzip stream that can't be decompressed chunk-by-chunk.
			req.Header.Del("Accept-Encoding")
		},
		FlushInterval: -1, // flush immediately for SSE/streaming responses
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

		ts := time.Now().Format("2006-01-02T15:04:05.000")
		proxyLog.Printf("=== REQUEST %s %s %s ===\n%s\n", ts, r.Method, r.URL.Path, reqBody)

		// Parse the request body
		parsed, parseErr := ParseRequestBody(reqBody)

		// Detect streaming mode
		var isStream bool
		if parseErr == nil {
			var streamCheck struct {
				Stream bool `json:"stream"`
			}
			json.Unmarshal(reqBody, &streamCheck)
			isStream = streamCheck.Stream
		}

		// Use a response recorder to capture the response
		rec := &responseCapture{ResponseWriter: w, statusCode: 200}
		p.rp.ServeHTTP(rec, r)

		proxyLog.Printf("=== RESPONSE %s status=%d stream=%v body_bytes=%d ===\n%s\n",
			ts, rec.statusCode, isStream, rec.body.Len(), rec.body.Bytes())

		// Process composition in background (don't block the response)
		if parseErr == nil && rec.statusCode >= 200 && rec.statusCode < 300 {
			// Extract session ID from x-session-id header if present, otherwise empty
			sessionID := r.Header.Get("x-session-id")
			go p.processComposition(parsed, rec.body.Bytes(), sessionID, isStream)
		}
	})
}

func (p *Proxy) processComposition(parsed ParseResult, respBody []byte, sessionID string, isStream bool) {
	var inputTokens int
	var err error

	if isStream {
		inputTokens, err = extractInputTokensFromSSE(respBody)
	} else {
		var respData struct {
			Usage struct {
				InputTokens              int `json:"input_tokens"`
				CacheReadInputTokens     int `json:"cache_read_input_tokens"`
				CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
			} `json:"usage"`
		}
		err = json.Unmarshal(respBody, &respData)
		inputTokens = respData.Usage.InputTokens +
			respData.Usage.CacheReadInputTokens +
			respData.Usage.CacheCreationInputTokens
	}
	if err != nil {
		log.Printf("proxy: parse response usage: %v", err)
		return
	}

	ctxSize := LookupContextWindowSize(parsed.Model, parsed.HasExtendedThinking)
	snap := Estimate(parsed, inputTokens, ctxSize)
	snap.SessionID = sessionID

	if err := PushComposition(p.webAddr, snap); err != nil {
		log.Printf("proxy: push composition: %v", err)
	}
}

// extractInputTokensFromSSE scans SSE event stream for message_start and
// extracts message.usage.input_tokens from its data line.
func extractInputTokensFromSSE(body []byte) (int, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	var foundMessageStart bool
	for scanner.Scan() {
		line := scanner.Text()
		if line == "event: message_start" {
			foundMessageStart = true
			continue
		}
		if foundMessageStart && strings.HasPrefix(line, "data: ") {
			var event struct {
				Message struct {
					Usage struct {
						InputTokens              int `json:"input_tokens"`
						CacheReadInputTokens     int `json:"cache_read_input_tokens"`
						CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
					} `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
				return 0, fmt.Errorf("parse message_start data: %w", err)
			}
			u := event.Message.Usage
			return u.InputTokens + u.CacheReadInputTokens + u.CacheCreationInputTokens, nil
		}
		if foundMessageStart && line == "" {
			// Empty line after event: means end of this event without data
			foundMessageStart = false
		}
	}
	return 0, fmt.Errorf("no message_start event found in SSE stream")
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

func (rc *responseCapture) Flush() {
	if f, ok := rc.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
