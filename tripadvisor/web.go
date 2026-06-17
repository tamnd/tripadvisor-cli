package tripadvisor

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// web.go holds the web-plane methods: they fetch a public location page and read
// its JSON-LD island. The web plane needs a real page URL, because a location id
// alone cannot rebuild the full SEO slug URL offline. The location id and geo id
// come from the URL via Classify, since the island does not carry them. A walled
// response becomes ErrBlocked in the client before any parser runs.

// WebLocation fetches a location page and parses its island into a Location. The
// ref must be a full TripAdvisor page URL or path; a bare id has no rebuildable web
// URL and is the API plane's job.
func (c *Client) WebLocation(ctx context.Context, ref string) (*Location, error) {
	pageURL, id, geo, err := c.webTarget(ref)
	if err != nil {
		return nil, err
	}
	body, err := c.get(ctx, pageURL)
	if err != nil {
		return nil, err
	}
	loc := locationFromLD(body, id, geo, pageURL)
	if loc == nil {
		return nil, ErrNotFound
	}
	return loc, nil
}

// WebReviews fetches a location page and lifts its embedded recent-review island.
func (c *Client) WebReviews(ctx context.Context, ref string, limit int) ([]*Review, error) {
	pageURL, id, _, err := c.webTarget(ref)
	if err != nil {
		return nil, err
	}
	body, err := c.get(ctx, pageURL)
	if err != nil {
		return nil, err
	}
	out := reviewsFromLD(body, id)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// WebPhotos is the thin web fallback for photos: it reads the location page's
// island and emits the single hero image it carries. The reliable source is the
// Content API; this returns at most one Photo so a keyless run still surfaces the
// page image rather than nothing.
func (c *Client) WebPhotos(ctx context.Context, ref string, limit int) ([]*Photo, error) {
	loc, err := c.WebLocation(ctx, ref)
	if err != nil {
		return nil, err
	}
	if loc.Image == "" {
		return nil, nil
	}
	return []*Photo{{
		ID:       loc.ID + "-hero",
		Location: loc.ID,
		Caption:  loc.Name,
		Original: loc.Image,
		Large:    loc.Image,
	}}, nil
}

// WebSearch is the best-effort web typeahead. It queries TripAdvisor's typeahead
// endpoint, which is behind the same DataDome wall as the rest of the web plane, so
// from a datacenter it returns ErrBlocked; from a residential or mobile connection
// it returns matches. The decoder is defensive: it fills only the fields the
// response actually carries and never fabricates a record for an empty result.
func (c *Client) WebSearch(ctx context.Context, query string, limit int) ([]*Location, error) {
	q := url.Values{}
	q.Set("action", "API")
	q.Set("query", query)
	q.Set("types", "geo,hotel,eat,attractions")
	if c.cfg.Language != "" {
		q.Set("hglang", c.cfg.Language)
	}
	body, err := c.get(ctx, c.cfg.BaseURL+"/TypeAheadJson?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var env struct {
		Results []struct {
			Result struct {
				LocationID json.Number `json:"location_id"`
				Name       string      `json:"name"`
				URL        string      `json:"url"`
				Category   struct {
					Name string `json:"name"`
				} `json:"category"`
				Coords string `json:"coords"`
			} `json:"result_object"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, ErrNotFound
	}
	var out []*Location
	for _, r := range env.Results {
		id := strings.TrimSpace(r.Result.LocationID.String())
		if id == "" || id == "0" {
			continue
		}
		loc := &Location{
			ID:       id,
			Name:     squish(r.Result.Name),
			Category: normalizeCategory(r.Result.Category.Name),
			URL:      strings.TrimSpace(r.Result.URL),
		}
		if lat, lng, ok := splitCoords(r.Result.Coords); ok {
			loc.Lat, loc.Lng = lat, lng
		}
		wireLocationEdges(loc)
		out = append(out, loc)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// webTarget turns a ref into the page URL to fetch and the (id, geo) the island
// will be tagged with. It needs a real page URL: a bare id cannot be turned into a
// web page offline, so it returns ErrUsage to send the caller to the API plane.
func (c *Client) webTarget(ref string) (pageURL, id, geo string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", "", ErrUsage
	}
	lower := strings.ToLower(ref)
	isURL := strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
	isPath := strings.Contains(ref, "/") || strings.Contains(ref, ".html")
	if !isURL && !isPath {
		// a bare id or g-number: no web page can be built offline
		return "", "", "", ErrUsage
	}
	r := Classify(ref)
	if r.Kind != "location" {
		return "", "", "", ErrUsage
	}
	id = r.ID
	if m := gRE.FindStringSubmatch("/" + strings.TrimLeft(ref, "/")); m != nil {
		geo = m[1]
	}
	if isURL {
		pageURL = ref
	} else {
		pageURL = c.cfg.BaseURL + "/" + strings.TrimLeft(ref, "/")
	}
	return pageURL, id, geo, nil
}

// splitCoords parses a "lat,lng" coordinate string.
func splitCoords(s string) (lat, lng float64, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(s), ",", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	la, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	ln, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return la, ln, true
}
