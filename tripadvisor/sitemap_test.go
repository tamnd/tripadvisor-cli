package tripadvisor

import (
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSitemaps(t *testing.T) {
	robots := "User-agent: *\n" +
		"Sitemap: https://www.tripadvisor.com/sitemap/att/en_US/sitemap_en_US_attractions_index.xml\n" +
		"Sitemap: https://www.tripadvisor.com/sitemap/2/en_US/sitemap_en_US_show_user_reviews_index.xml\n" +
		"Sitemap: https://www.tripadvisor.com/business/sitemap.xml\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(robots))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	idxs, err := c.Sitemaps(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(idxs) != 3 {
		t.Fatalf("got %d indexes, want 3", len(idxs))
	}
	if idxs[0].Kind != "attractions" || idxs[0].Category != "place" {
		t.Errorf("idx0 = %+v", idxs[0])
	}
	if idxs[1].Kind != "show_user_reviews" || idxs[1].Category != "review" {
		t.Errorf("idx1 = %+v", idxs[1])
	}
	if idxs[2].Kind != "business" || idxs[2].Category != "business" {
		t.Errorf("idx2 = %+v", idxs[2])
	}
}

func TestSitemapSeeds(t *testing.T) {
	// The index points at one gzipped shard; the shard enumerates two attraction
	// pages, one with a d-number (so it carries the location edge).
	shard := `<?xml version="1.0"?><urlset>
 <url><loc>https://www.tripadvisor.com/Attraction_Review-g187147-d188757-Reviews-Eiffel_Tower.html</loc><lastmod>2026-05-01</lastmod></url>
 <url><loc>https://www.tripadvisor.com/Attractions-g187147-Activities-Paris.html</loc></url>
</urlset>`

	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap/att/en_US/sitemap_en_US_attractions_index.xml", func(w http.ResponseWriter, r *http.Request) {
		// Point the shard URL back at this test server.
		idx := `<?xml version="1.0"?><sitemapindex><sitemap><loc>` +
			baseOf(r) + `/shard.xml.gz</loc></sitemap></sitemapindex>`
		_, _ = w.Write([]byte(idx))
	})
	mux.HandleFunc("/shard.xml.gz", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write([]byte(shard))
		_ = zw.Close()
		_, _ = w.Write(buf.Bytes())
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := testClient(srv.URL)
	seeds, err := c.Sitemap(context.Background(), "attractions", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(seeds) != 2 {
		t.Fatalf("got %d seeds, want 2", len(seeds))
	}
	if seeds[0].Location != "188757" {
		t.Errorf("seed0 location edge = %q, want 188757", seeds[0].Location)
	}
	if seeds[0].Lastmod != "2026-05-01" {
		t.Errorf("seed0 lastmod = %q", seeds[0].Lastmod)
	}
	if seeds[1].Location != "187147" {
		t.Errorf("seed1 (geo) location edge = %q, want 187147", seeds[1].Location)
	}
}

func TestMaybeGunzip(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write([]byte("hello"))
	_ = zw.Close()
	if got := maybeGunzip(buf.Bytes()); string(got) != "hello" {
		t.Errorf("gunzip = %q", got)
	}
	if got := maybeGunzip([]byte("plain")); string(got) != "plain" {
		t.Errorf("passthrough = %q", got)
	}
}

func baseOf(r *http.Request) string {
	scheme := "http"
	return scheme + "://" + r.Host
}
