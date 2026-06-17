package tripadvisor

import (
	"net/url"
	"regexp"
	"strings"
)

// ids.go is the offline reference layer: Classify turns any TripAdvisor URL, path,
// or bare id into a canonical (kind, id), and URLFor builds an addressable URL for
// a (kind, id). Both are pure and never touch the network, so `ta ref id` and `ta
// ref url` (and a host's resolve/url) answer instantly.
//
// The d-number in a page URL is the canonical location id and the same id the
// Content API uses. The g-number is the geo id, kept for the ancestor edge and for
// rebuilding a geo URL. A geo (a city, region, or country) is a location whose
// category is geo, addressed by its g-number.
//
// The kinds:
//   - location: a hotel/restaurant/attraction by d-number, or a geo by g-number
//   - reviews/photos/nearby: list authorities derived from a location id
//   - search: a free-text query
//   - sitemap: one per-kind sitemap index, id the kind token
//   - sitemaps: the robots.txt master list, no id

var (
	dRE = regexp.MustCompile(`[-/]d(\d+)`)
	gRE = regexp.MustCompile(`[-/]g(\d+)`)
	// sitemapIndexRE captures the kind from a /sitemap/.../sitemap_en_US_<kind>_index.xml path.
	sitemapIndexRE = regexp.MustCompile(`sitemap_en_US_(.+)_index\.xml$`)
	numRE          = regexp.MustCompile(`^\d+$`)
	geoRE          = regexp.MustCompile(`^g(\d+)$`)
	kindRE         = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)
)

// sitemapSection maps a per-kind sitemap index token to the URL section it lives
// under, seeded from the eight indexes robots.txt advertises. A kind not in the
// table has no rebuildable index URL; run sitemaps to discover it.
var sitemapSection = map[string]string{
	"location_photo_direct_link": "2",
	"show_user_reviews":          "2",
	"attractions":                "att",
	"attraction_review":          "att",
	"attraction_product_review":  "att",
	"attractions_near":           "att",
}

// Classify resolves a reference offline. It accepts a full TripAdvisor URL, a
// path, or a bare id ("93450", "g60763").
func Classify(input string) Ref {
	in := strings.TrimSpace(input)
	r := Ref{Input: input, Kind: "unknown"}

	path := in
	wasURL := false
	if u, err := url.Parse(in); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		path = u.Path
		wasURL = true
	}
	clean := strings.Trim(path, "/")

	switch {
	case clean == "robots.txt":
		r.Kind, r.ID = "sitemaps", ""
	case sitemapIndexRE.MatchString(clean):
		kind := sitemapIndexRE.FindStringSubmatch(clean)[1]
		if kindRE.MatchString(kind) {
			r.Kind, r.ID = "sitemap", kind
		}
	case strings.HasPrefix(clean, "business/sitemap"):
		r.Kind, r.ID = "sitemap", "business"
	case dRE.MatchString("/" + clean):
		// a place page: the d-number is the canonical id
		r.Kind = "location"
		r.ID = dRE.FindStringSubmatch("/" + clean)[1]
	case gRE.MatchString("/" + clean):
		// a geo landing page (Tourism/Hotels/Restaurants/Attractions): the g-number
		r.Kind = "location"
		r.ID = gRE.FindStringSubmatch("/" + clean)[1]
	case numRE.MatchString(clean):
		r.Kind, r.ID = "location", clean
	case geoRE.MatchString(clean):
		r.Kind, r.ID = "location", geoRE.FindStringSubmatch(clean)[1]
	}

	if r.Kind != "unknown" {
		// When the input was a full page URL, echo it back; it is the human page and
		// is more useful than a rebuilt API URL. Otherwise build what we can offline.
		if wasURL {
			r.URL = in
		} else {
			r.URL = URLFor(r.Kind, r.ID)
		}
	}
	return r
}

// URLFor builds an addressable URL for a (kind, id), or "" if it cannot. A bare
// location id cannot rebuild the full SEO page URL offline (it needs the geo and
// the slug), so location/reviews/photos return the stable Content API URL as the
// addressable form; a record's own URL carries the human page after a fetch.
func URLFor(kind, id string) string {
	id = strings.Trim(id, "/")
	switch kind {
	case "location":
		if id == "" {
			return ""
		}
		return ContentURL + "/location/" + id + "/details"
	case "reviews":
		if id == "" {
			return ""
		}
		return ContentURL + "/location/" + id + "/reviews"
	case "photos":
		if id == "" {
			return ""
		}
		return ContentURL + "/location/" + id + "/photos"
	case "nearby":
		if id == "" {
			return ""
		}
		return ContentURL + "/location/nearby_search?latLong=" + url.QueryEscape(id)
	case "search":
		if id == "" {
			return ""
		}
		return ContentURL + "/location/search?searchQuery=" + url.QueryEscape(id)
	case "sitemap":
		return sitemapURL(id)
	case "sitemaps":
		return BaseURL + "/robots.txt"
	default:
		return ""
	}
}

// sitemapURL rebuilds a per-kind sitemap index URL from the section table, or ""
// for a kind not in it.
func sitemapURL(kind string) string {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		return ""
	}
	if kind == "business" {
		return BaseURL + "/business/sitemap.xml"
	}
	section, ok := sitemapSection[kind]
	if !ok {
		return ""
	}
	return BaseURL + "/sitemap/" + section + "/en_US/sitemap_en_US_" + kind + "_index.xml"
}

// sitemapCategory groups a sitemap kind into the kind of page it covers, so a crawl
// can pick the backbones it cares about.
func sitemapCategory(kind string) string {
	switch kind {
	case "attractions", "attractions_near":
		return "place"
	case "attraction_review", "attraction_product_review":
		return "attraction"
	case "show_user_reviews":
		return "review"
	case "location_photo_direct_link":
		return "photo"
	case "business":
		return "business"
	default:
		return "other"
	}
}
