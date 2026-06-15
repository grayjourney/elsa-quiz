//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// restClient is a thin black-box HTTP client for the quiz control plane.
type restClient struct {
	base string
	http *http.Client
}

func newREST(base string) *restClient {
	return &restClient{base: base, http: &http.Client{Timeout: 10 * time.Second}}
}

// do issues a request and returns the status code and raw body. userID, when
// non-empty, is sent as the X-User-ID header (the server's mocked identity).
func (c *restClient) do(method, path, userID string, body any) (int, []byte, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, err
}
