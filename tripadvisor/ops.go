package tripadvisor

import (
	"context"

	"github.com/tamnd/any-cli/kit/errs"
)

// ops.go holds the handler for every operation declared in domain.go. kit reflects
// each input struct into CLI flags, HTTP query params, and MCP tool arguments:
// kit:"arg" is a positional, kit:"flag,inherit" binds the shared --limit, and
// kit:"inject" receives the client newClient builds. The reference ops (id, url)
// take no client; they run offline.
//
// Each handler that both planes cover resolves the plane once and calls the
// matching method. The record type is identical either way, so the output,
// --fields, and the edges do not change with the plane; only which fields are
// filled changes, and omitempty carries the difference. A chosen plane that is
// walled or keyless returns its error and exits; no plane silently falls back to
// the other.

// planeFor resolves which plane an op runs on. webOK reports the op has a web
// method, apiOK that it has an API method. The rules follow the yelp model: a key
// prefers the API, no key uses the web, --plane overrides, and a forced plane the
// op cannot serve is an auth error naming the key.
func (c *Client) planeFor(webOK, apiOK bool) (string, error) {
	switch c.cfg.Plane {
	case "web":
		if !webOK {
			return "", ErrNeedKey
		}
		return "web", nil
	case "api":
		if !apiOK || !c.cfg.hasKey() {
			return "", ErrNeedKey
		}
		return "api", nil
	default: // auto
		if apiOK && c.cfg.hasKey() {
			return "api", nil
		}
		if webOK {
			return "web", nil
		}
		return "", ErrNeedKey
	}
}

// --- search ---

type searchIn struct {
	Query  string  `kit:"arg" help:"a name to search, e.g. \"Eiffel Tower\""`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func search(ctx context.Context, in searchIn, emit func(*Location) error) error {
	plane, err := in.Client.planeFor(true, true)
	if err != nil {
		return mapErr(err)
	}
	n := limitOr(in.Limit, defaultLimit)
	var items []*Location
	if plane == "api" {
		items, err = in.Client.SearchAPI(ctx, in.Query, n)
	} else {
		items, err = in.Client.WebSearch(ctx, in.Query, n)
	}
	if err != nil {
		return mapErr(err)
	}
	return emitAll(items, emit)
}

// --- location ---

type locationIn struct {
	Ref    string  `kit:"arg" help:"a location id, or a TripAdvisor page URL"`
	Client *Client `kit:"inject"`
}

func getLocation(ctx context.Context, in locationIn, emit func(*Location) error) error {
	plane, err := in.Client.planeFor(true, true)
	if err != nil {
		return mapErr(err)
	}
	var loc *Location
	if plane == "api" {
		loc, err = in.Client.DetailsAPI(ctx, refID(in.Ref))
	} else {
		loc, err = in.Client.WebLocation(ctx, in.Ref)
	}
	if err != nil {
		return mapErr(err)
	}
	return emit(loc)
}

// --- reviews ---

type reviewsIn struct {
	Ref    string  `kit:"arg" help:"a location id, or a TripAdvisor page URL"`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func reviews(ctx context.Context, in reviewsIn, emit func(*Review) error) error {
	plane, err := in.Client.planeFor(true, true)
	if err != nil {
		return mapErr(err)
	}
	n := limitOr(in.Limit, defaultLimit)
	var rs []*Review
	if plane == "api" {
		rs, err = in.Client.ReviewsAPI(ctx, refID(in.Ref), n)
	} else {
		rs, err = in.Client.WebReviews(ctx, in.Ref, n)
	}
	if err != nil {
		return mapErr(err)
	}
	return emitAll(rs, emit)
}

// --- photos ---

type photosIn struct {
	Ref    string  `kit:"arg" help:"a location id, or a TripAdvisor page URL"`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func photos(ctx context.Context, in photosIn, emit func(*Photo) error) error {
	plane, err := in.Client.planeFor(true, true)
	if err != nil {
		return mapErr(err)
	}
	n := limitOr(in.Limit, defaultLimit)
	var ps []*Photo
	if plane == "api" {
		ps, err = in.Client.PhotosAPI(ctx, refID(in.Ref), n)
	} else {
		ps, err = in.Client.WebPhotos(ctx, in.Ref, n)
	}
	if err != nil {
		return mapErr(err)
	}
	return emitAll(ps, emit)
}

// --- nearby (api-only) ---

type nearbyIn struct {
	LatLong string  `kit:"arg" help:"a point as \"lat,long\", e.g. \"48.8584,2.2945\""`
	Limit   int     `kit:"flag,inherit"`
	Client  *Client `kit:"inject"`
}

func nearby(ctx context.Context, in nearbyIn, emit func(*Location) error) error {
	plane, err := in.Client.planeFor(false, true)
	if err != nil {
		return mapErr(err)
	}
	_ = plane // only the api plane reaches here
	items, err := in.Client.NearbyAPI(ctx, in.LatLong, limitOr(in.Limit, defaultLimit))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(items, emit)
}

// --- sitemaps (web, crawl root) ---

type sitemapsIn struct {
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func sitemaps(ctx context.Context, in sitemapsIn, emit func(*SitemapIndex) error) error {
	idxs, err := in.Client.Sitemaps(ctx, limitOr(in.Limit, 0))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(idxs, emit)
}

// --- sitemap (web, crawl root) ---

type sitemapIn struct {
	Kind   string  `kit:"arg" help:"a sitemap kind, e.g. attractions; run sitemaps to list them all"`
	Limit  int     `kit:"flag,inherit"`
	Client *Client `kit:"inject"`
}

func sitemap(ctx context.Context, in sitemapIn, emit func(*Seed) error) error {
	seeds, err := in.Client.Sitemap(ctx, in.Kind, limitOr(in.Limit, defaultLimit))
	if err != nil {
		return mapErr(err)
	}
	return emitAll(seeds, emit)
}

// --- reference tools (offline) ---

type refIn struct {
	Ref string `kit:"arg" help:"any TripAdvisor URL, path, or id"`
}

func classifyRef(_ context.Context, in refIn, emit func(*Ref) error) error {
	r := Classify(in.Ref)
	if r.Kind == "unknown" {
		return errs.Usage("unrecognized tripadvisor reference: %q", in.Ref)
	}
	return emit(&r)
}

type urlIn struct {
	Kind string `kit:"arg" help:"location, reviews, photos, nearby, search, sitemap, or sitemaps"`
	ID   string `kit:"arg" help:"the id for that kind"`
}

func buildURL(_ context.Context, in urlIn, emit func(*Ref) error) error {
	u := URLFor(in.Kind, in.ID)
	if u == "" {
		return errs.Usage("tripadvisor cannot build a URL for %q/%q", in.Kind, in.ID)
	}
	return emit(&Ref{Input: in.Kind + "/" + in.ID, Kind: in.Kind, ID: in.ID, URL: u})
}

// --- helpers ---

// refID returns the location id for a ref: a bare id passes through, a URL or path
// is classified to its d/g-number.
func refID(ref string) string {
	r := Classify(ref)
	if r.Kind == "location" && r.ID != "" {
		return r.ID
	}
	return ref
}

// emitAll streams a slice of records through emit.
func emitAll[T any](items []*T, emit func(*T) error) error {
	for _, it := range items {
		if err := emit(it); err != nil {
			return err
		}
	}
	return nil
}

// limitOr returns the operator's --limit when set, else the command's own default
// fetch count.
func limitOr(limit, def int) int {
	if limit > 0 {
		return limit
	}
	return def
}
