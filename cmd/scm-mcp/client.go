package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"scm/pkg/aksk"
)

// Client is a signed HTTP client for the SCM backend. Every request carries
// the AK/SK header triple so the server can authenticate and authorize the
// agent against the key's tenant + permission set.
type Client struct {
	baseURL string
	ak, sk  string
	http    *http.Client
}

func NewClient(baseURL, ak, sk string) *Client {
	return &Client{baseURL: baseURL, ak: ak, sk: sk, http: &http.Client{Timeout: 30 * time.Second}}
}

// call performs a signed request and returns the raw `data` payload of the
// {code,message,data} envelope. path must be the URL path WITHOUT the query
// string (the signature does not cover the query); query params are appended
// to the request URL only.
func (c *Client) call(method, path string, query url.Values, body any) (json.RawMessage, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := aksk.Sign(c.ak, ts, method, path, aksk.SHA256Hex(bodyBytes), c.sk)

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(method, u, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(aksk.HeaderKey, c.ak)
	req.Header.Set(aksk.HeaderTimestamp, ts)
	req.Header.Set(aksk.HeaderSignature, sig)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var env struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("invalid response: %s", string(raw))
	}
	if env.Code != 0 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("code=%d %s", env.Code, env.Message)
	}
	return env.Data, nil
}

// pretty renders raw JSON as indented text for a tool result.
func pretty(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "{}"
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(b)
}
