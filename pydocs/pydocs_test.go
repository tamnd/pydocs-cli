package pydocs_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/pydocs-cli/pydocs"
)

const testHTML = `
     <tr>
       <td></td>
       <td>
       <a href="library/__future__.html#module-__future__"><code class="xref">__future__</code></a></td><td>
       <em>Future statement definitions</em></td></tr>
     <tr>
       <td></td>
       <td>
       <a href="library/abc.html#module-abc"><code class="xref">abc</code></a></td><td>
       <em>Abstract base classes according to :pep:3119.</em></td></tr>
     <tr>
       <td></td>
       <td>
       <a href="library/asyncio.html#module-asyncio"><code class="xref">asyncio</code></a></td><td>
       <em>Asynchronous I/O.</em></td></tr>
`

func newTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte(body))
	}))
}

func TestList(t *testing.T) {
	srv := newTestServer(t, testHTML)
	defer srv.Close()

	cfg := pydocs.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0

	c := pydocs.NewClient(cfg)
	mods, err := c.List(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 3 {
		t.Fatalf("got %d modules, want 3", len(mods))
	}
	if mods[0].Name != "__future__" {
		t.Errorf("Name = %q, want __future__", mods[0].Name)
	}
	if mods[0].Description != "Future statement definitions" {
		t.Errorf("Description = %q", mods[0].Description)
	}
	if !contains(mods[0].URL, "library/__future__.html") {
		t.Errorf("URL = %q, want to contain library/__future__.html", mods[0].URL)
	}
}

func TestListLimit(t *testing.T) {
	srv := newTestServer(t, testHTML)
	defer srv.Close()

	cfg := pydocs.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0

	c := pydocs.NewClient(cfg)
	mods, err := c.List(context.Background(), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) != 2 {
		t.Fatalf("got %d modules, want 2 (limit applied)", len(mods))
	}
}

func TestSearch(t *testing.T) {
	srv := newTestServer(t, testHTML)
	defer srv.Close()

	cfg := pydocs.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0

	c := pydocs.NewClient(cfg)
	results, err := c.Search(context.Background(), "async", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Name != "asyncio" {
		t.Errorf("Name = %q, want asyncio", results[0].Name)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
