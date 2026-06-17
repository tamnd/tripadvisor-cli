package tripadvisor

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"io"
	"regexp"
	"strings"
)

// sitemap.go reads TripAdvisor's published sitemaps, the reconstruction backbone
// for a crawl. robots.txt advertises a per-section index; each index is a
// sitemapindex of shards; each shard is a urlset (often gzipped) enumerating
// landing pages. Two operations read this:
//
//   - sitemaps walks robots.txt and lists every advertised index, so a crawl can
//     discover the whole backbone with no prior knowledge of what kinds exist.
//   - sitemap reads one kind's index, gunzips and parses its shards, and emits a
//     Seed per page.
//
// The index path is the sectioned /sitemap/<section>/en_US/sitemap_en_US_<kind>_index.xml
// form, so the kind -> URL mapping is the small table in ids.go seeded from the
// eight advertised indexes, with sitemaps as the discovery root for anything else.
//
// The sitemaps live behind the same DataDome wall as the rest of the web plane, so
// from a datacenter they return ErrBlocked; from a residential or mobile connection
// they read. A seed always emits (complete inventory) and fills the location edge
// when Classify resolves a d-number from the URL.

// robotsSitemapRE pulls each advertised index URL out of robots.txt.
var robotsSitemapRE = regexp.MustCompile(`(?mi)^\s*Sitemap:\s*(\S+)\s*$`)

// xmlIndex is a sitemapindex: a list of shard URLs.
type xmlIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

// xmlURLSet is a urlset: the entries of one shard.
type xmlURLSet struct {
	URLs []struct {
		Loc     string `xml:"loc"`
		Lastmod string `xml:"lastmod"`
	} `xml:"url"`
}

// Sitemaps lists the sitemap indexes advertised in robots.txt, the root of the
// backbone. SeedsRef links into the sitemap op for the kind, so a crawl walks from
// here into the seeds.
func (c *Client) Sitemaps(ctx context.Context, limit int) ([]*SitemapIndex, error) {
	body, err := c.get(ctx, c.cfg.BaseURL+"/robots.txt")
	if err != nil {
		return nil, err
	}
	var out []*SitemapIndex
	seen := map[string]bool{}
	for _, m := range robotsSitemapRE.FindAllSubmatch(body, -1) {
		loc := strings.TrimSpace(string(m[1]))
		if loc == "" || seen[loc] {
			continue
		}
		seen[loc] = true
		idx := &SitemapIndex{URL: loc}
		if kind := indexKind(loc); kind != "" {
			idx.Kind = kind
			idx.Category = sitemapCategory(kind)
			idx.SeedsRef = kind
		}
		out = append(out, idx)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// indexKind returns the kind a sitemap index URL enumerates, or "" for an index
// whose name does not fit the per-kind template (the language master index).
func indexKind(loc string) string {
	name := loc
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if strings.HasPrefix(loc, BaseURL+"/business/") || strings.Contains(loc, "/business/sitemap") {
		return "business"
	}
	m := sitemapIndexRE.FindStringSubmatch(name)
	if m == nil || !kindRE.MatchString(m[1]) || m[1] == "" {
		return ""
	}
	return m[1]
}

// Sitemap returns up to limit seeds for a kind by reading its index and shards. It
// is the crawl root: every seed names a live landing page and, when the page maps
// to a location, the edge into it.
func (c *Client) Sitemap(ctx context.Context, kind string, limit int) ([]*Seed, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if !kindRE.MatchString(kind) {
		return nil, ErrUsage
	}
	indexURL := c.sitemapIndexURL(kind)
	if indexURL == "" {
		return nil, ErrUsage
	}
	body, err := c.get(ctx, indexURL)
	if err != nil {
		return nil, err
	}
	var idx xmlIndex
	if err := xml.Unmarshal(maybeGunzip(body), &idx); err != nil {
		return nil, ErrNotFound
	}

	// A few indexes (business) are a plain urlset rather than a sitemapindex; try
	// that shape when no shards were found.
	if len(idx.Sitemaps) == 0 {
		return seedsFromURLSet(maybeGunzip(body), kind, limit), nil
	}

	var out []*Seed
	seen := map[string]bool{}
	for _, s := range idx.Sitemaps {
		shardURL := strings.TrimSpace(s.Loc)
		if shardURL == "" {
			continue
		}
		sb, err := c.get(ctx, shardURL)
		if err != nil {
			// One bad shard should not sink the walk; move to the next.
			continue
		}
		var set xmlURLSet
		if err := xml.Unmarshal(maybeGunzip(sb), &set); err != nil {
			continue
		}
		for _, u := range set.URLs {
			loc := strings.TrimSpace(u.Loc)
			if loc == "" || seen[loc] {
				continue
			}
			seen[loc] = true
			out = append(out, seedFor(kind, loc, strings.TrimSpace(u.Lastmod)))
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

// sitemapIndexURL rebuilds a per-kind index URL against the configured base, so a
// test can point it at a local server. It mirrors sitemapURL (which uses the public
// BaseURL for the offline ref tools) with cfg.BaseURL.
func (c *Client) sitemapIndexURL(kind string) string {
	u := sitemapURL(kind)
	if u == "" {
		return ""
	}
	if c.cfg.BaseURL != "" && c.cfg.BaseURL != BaseURL {
		return c.cfg.BaseURL + strings.TrimPrefix(u, BaseURL)
	}
	return u
}

// seedsFromURLSet parses a body that is itself a urlset into seeds.
func seedsFromURLSet(body []byte, kind string, limit int) []*Seed {
	var set xmlURLSet
	if err := xml.Unmarshal(body, &set); err != nil {
		return nil
	}
	var out []*Seed
	seen := map[string]bool{}
	for _, u := range set.URLs {
		loc := strings.TrimSpace(u.Loc)
		if loc == "" || seen[loc] {
			continue
		}
		seen[loc] = true
		out = append(out, seedFor(kind, loc, strings.TrimSpace(u.Lastmod)))
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// seedFor builds a Seed from a sitemap entry. It always emits the page, since the
// point of the backbone is a complete URL inventory, and wires the location edge
// when Classify resolves a d-number or geo from the URL.
func seedFor(kind, loc, lastmod string) *Seed {
	r := Classify(loc)
	s := &Seed{Kind: kind, ID: r.ID, URL: loc, Lastmod: lastmod}
	if r.Kind == "location" {
		s.Location = r.ID
	}
	return s
}

// maybeGunzip returns the gzip-decompressed body when it carries the gzip magic
// bytes, else the body unchanged, so a .gz shard and a plain .xml index both parse
// through one path.
func maybeGunzip(b []byte) []byte {
	if len(b) < 2 || b[0] != 0x1f || b[1] != 0x8b {
		return b
	}
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return b
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		return b
	}
	return out
}
