// Package tripadvisor is the library behind the ta command line: the HTTP
// client for both planes, the offline reference layer, and the typed records read
// from public TripAdvisor surfaces.
//
// TripAdvisor has two planes. The web plane (www.tripadvisor.com) is the default
// and reads the public site, but it is fronted by DataDome, so it is best-effort:
// a walled response becomes ErrBlocked before any parser runs. The Content API
// plane (api.content.tripadvisor.com) is the opt-in upgrade, turned on by a
// TRIPADVISOR_API_KEY in the environment, and reads reliably from anywhere. The
// Client below GETs both, paces and retries politely, caches on disk, and turns a
// walled or rejected response into a typed error the exit-code mapping understands.
package tripadvisor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client reads public TripAdvisor data over HTTP on both planes.
type Client struct {
	HTTP *http.Client
	cfg  Config
	last time.Time
}

// NewClient returns a Client configured from cfg, filling unset fields with
// their defaults.
func NewClient(cfg Config) *Client {
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = BaseURL
	}
	if cfg.ContentURL == "" {
		cfg.ContentURL = ContentURL
	}
	return &Client{
		HTTP: &http.Client{Timeout: cfg.Timeout},
		cfg:  cfg,
	}
}

// get fetches a web-plane URL and returns the body. It serves from cache when
// fresh, paces and retries transient failures, and classifies a walled response
// as ErrBlocked.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	return c.fetch(ctx, rawURL, false)
}

// getAPI fetches a Content API URL. The api error mapping differs (a 401 is the
// rejected/missing key, not a 403 challenge), and the cache key strips the secret.
func (c *Client) getAPI(ctx context.Context, rawURL string) ([]byte, error) {
	return c.fetch(ctx, rawURL, true)
}

func (c *Client) fetch(ctx context.Context, rawURL string, api bool) ([]byte, error) {
	ckey := rawURL
	if api {
		ckey = stripKey(rawURL)
	}
	if b := c.cacheGet(ckey); b != nil {
		return b, nil
	}
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL, api)
		if err == nil {
			c.cachePut(ckey, body)
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	if errors.Is(lastErr, ErrRateLimited) {
		return nil, ErrRateLimited
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string, api bool) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	if api {
		req.Header.Set("Accept", "application/json")
	} else {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml")
		if c.cfg.Language != "" {
			req.Header.Set("Accept-Language", c.cfg.Language)
		}
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		// A reset or handshake failure mid-request is retryable; the fetch loop
		// turns a persistent failure into the wrapped error.
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case api && resp.StatusCode == http.StatusUnauthorized:
		return nil, false, ErrBlocked
	case resp.StatusCode == http.StatusForbidden:
		return nil, false, ErrBlocked
	case resp.StatusCode == http.StatusNotFound:
		return nil, false, ErrNotFound
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, true, ErrRateLimited
	case resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	if !api && isChallenge(b) {
		return nil, false, ErrBlocked
	}
	return b, false, nil
}

// pace blocks until at least Delay has passed since the previous request.
func (c *Client) pace() {
	if c.cfg.Delay <= 0 {
		return
	}
	if wait := c.cfg.Delay - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// challengeMarkers are byte signatures of a DataDome interstitial served with a
// 200 in place of the real page.
var challengeMarkers = [][]byte{
	[]byte("captcha-delivery.com"),
	[]byte("var dd="),
	[]byte("please enable js and disable any ad blocker"),
	[]byte("geo.captcha-delivery.com"),
}

// isChallenge reports whether a 200 body is a DataDome challenge rather than a
// real page, by looking for a known marker in the head of the body.
func isChallenge(body []byte) bool {
	head := body
	if len(head) > 8192 {
		head = head[:8192]
	}
	lower := bytes.ToLower(head)
	for _, m := range challengeMarkers {
		if bytes.Contains(lower, m) {
			return true
		}
	}
	return false
}

// squish collapses internal whitespace and trims, for text pulled out of HTML.
func squish(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
