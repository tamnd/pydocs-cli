// Package pydocs is the library behind the pydocs command line:
// the HTTP client, request shaping, and the typed data models for the
// Python standard library module index from docs.python.org.
package pydocs

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to docs.python.org.
const DefaultUserAgent = "Mozilla/5.0 (compatible; pydocs-cli/0.1; +https://github.com/tamnd/pydocs-cli)"

// modRe matches a module entry in the py-modindex.html page.
// Groups: [1] href path, [2] module name, [3] description.
var modRe = regexp.MustCompile(`href="(library/[^"]+\.html[^"]*)"><code[^>]*>([^<]+)</code></a></td><td>\s*<em>([^<]*)</em>`)

// Module is a Python standard library module.
type Module struct {
	Rank        int    `json:"rank"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// Config holds constructor parameters.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://docs.python.org",
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client talks to docs.python.org for Python standard library modules.
type Client struct {
	cfg        Config
	httpClient *http.Client
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		b, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return b, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get: %w", lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
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

// parseModules extracts modules from the py-modindex.html page HTML.
func parseModules(html string, limit int) []Module {
	matches := modRe.FindAllStringSubmatch(html, -1)
	var out []Module
	rank := 0
	for _, m := range matches {
		href := m[1]
		name := m[2]
		desc := strings.TrimSpace(m[3])
		rank++
		if limit > 0 && rank > limit {
			break
		}
		out = append(out, Module{
			Rank:        rank,
			Name:        name,
			Description: desc,
			URL:         "https://docs.python.org/3/" + href,
		})
	}
	return out
}

// List fetches all Python standard library modules.
func (c *Client) List(ctx context.Context, limit int) ([]Module, error) {
	raw, err := c.get(ctx, c.cfg.BaseURL+"/3/py-modindex.html")
	if err != nil {
		return nil, err
	}
	return parseModules(string(raw), limit), nil
}

// Search searches modules by name or description (client-side filtering).
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Module, error) {
	all, err := c.List(ctx, 0)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []Module
	rank := 0
	for _, mod := range all {
		if strings.Contains(strings.ToLower(mod.Name), q) || strings.Contains(strings.ToLower(mod.Description), q) {
			rank++
			mod.Rank = rank
			out = append(out, mod)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}
