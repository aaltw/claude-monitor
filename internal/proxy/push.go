package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var pushClient = &http.Client{Timeout: 1 * time.Second}

// PushComposition sends a ContextSnapshot to the web server's internal endpoint.
func PushComposition(webAddr string, snap ContextSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	resp, err := pushClient.Post(webAddr+"/api/internal/context", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("push to web server: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("web server returned %d", resp.StatusCode)
	}
	return nil
}
