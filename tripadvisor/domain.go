package tripadvisor

import (
	"context"
	"errors"
	"time"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes tripadvisor as a kit Domain: a driver that a multi-domain host
// (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/tripadvisor-cli/tripadvisor"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// tripadvisor:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone ta binary (see cli.NewApp), so the binary and a
// host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the tripadvisor driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme:   "tripadvisor",
		Hosts:    []string{Host, "tripadvisor.com"},
		Identity: Identity(),
	}
}

// Identity is the fixed description of the TripAdvisor CLI, shared by the domain
// and the standalone composition root so help and version read the same.
func Identity() kit.Identity {
	return kit.Identity{
		Binary: "ta",
		Short:  "Read public TripAdvisor locations, reviews, photos, and sitemaps into structured records",
		Long: `ta reads public TripAdvisor data over plain HTTPS in two planes. The
web plane reads www.tripadvisor.com and is the default, but it is fronted
by DataDome, so from a datacenter it is best-effort and a wall returns
exit 4; it works from a residential or mobile connection. The Content API
plane reads api.content.tripadvisor.com reliably from anywhere and turns
on when TRIPADVISOR_API_KEY is set in the environment (a free key, never a
flag). The two planes share one location id and one record shape, so a web
read and an API read address the same record. ta returns records as a
table, JSON, JSONL, CSV, TSV, or URLs, and serves the same operations over
HTTP and MCP.

ta is an independent tool and is not affiliated with TripAdvisor.`,
		Site: BaseURL,
		Repo: "https://github.com/tamnd/tripadvisor-cli",
	}
}

// Register installs the client factory and every operation onto app. The read
// group is the data; the crawl group is the reconstruction backbone, kept apart
// because its records are pointers into the graph rather than data at a node; the
// ref group is offline and never touches the network.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)
	app.CommandGroup("read", "Read public TripAdvisor data")
	app.CommandGroup("crawl", "Enumerate the sitemap backbone (web plane)")
	app.CommandGroup("ref", "Resolve references to ids and URLs (offline)")

	kit.Handle(app, kit.OpMeta{
		Name: "search", Group: "read", List: true,
		Summary: "Search locations by name (API when keyed, best-effort web typeahead otherwise)",
		URIType: "search",
		Args:    []kit.Arg{{Name: "query", Help: "a name to search, e.g. \"Eiffel Tower\""}},
	}, search)

	kit.Handle(app, kit.OpMeta{
		Name: "location", Group: "read", Single: true,
		Summary: "Show one location in full (web island by URL, API details by id)",
		URIType: "location", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "a location id, or a TripAdvisor page URL"}},
	}, getLocation)

	kit.Handle(app, kit.OpMeta{
		Name: "reviews", Group: "read", List: true,
		Summary: "List a location's reviews",
		URIType: "reviews",
		Args:    []kit.Arg{{Name: "ref", Help: "a location id, or a TripAdvisor page URL"}},
	}, reviews)

	kit.Handle(app, kit.OpMeta{
		Name: "photos", Group: "read", List: true,
		Summary: "List a location's photos (API plane; the web island carries only the hero image)",
		URIType: "photos",
		Args:    []kit.Arg{{Name: "ref", Help: "a location id, or a TripAdvisor page URL"}},
	}, photos)

	kit.Handle(app, kit.OpMeta{
		Name: "nearby", Group: "read", List: true,
		Summary: "List locations near a point (API plane only; needs TRIPADVISOR_API_KEY)",
		URIType: "nearby",
		Args:    []kit.Arg{{Name: "latlong", Help: "a point as \"lat,long\", e.g. \"48.8584,2.2945\""}},
	}, nearby)

	kit.Handle(app, kit.OpMeta{
		Name: "sitemaps", Group: "crawl", List: true,
		Summary: "List the sitemap indexes TripAdvisor advertises in robots.txt",
		URIType: "sitemaps",
	}, sitemaps)

	kit.Handle(app, kit.OpMeta{
		Name: "sitemap", Group: "crawl", List: true,
		Summary: "Enumerate a kind's pages from its sitemap index (the crawl root)",
		URIType: "sitemap",
		Args:    []kit.Arg{{Name: "kind", Help: "a sitemap kind, e.g. attractions; run sitemaps to list them all"}},
	}, sitemap)

	// Reference tools (offline).
	kit.Handle(app, kit.OpMeta{
		Name: "id", Parent: "ref", Single: true,
		Summary: "Classify a reference into its (kind, id)",
		Args:    []kit.Arg{{Name: "ref", Help: "any TripAdvisor URL, path, or id"}},
	}, classifyRef)

	kit.Handle(app, kit.OpMeta{
		Name: "url", Parent: "ref", Single: true,
		Summary: "Build the addressable URL for a (kind, id)",
		Args: []kit.Arg{
			{Name: "kind", Help: "location, reviews, photos, nearby, search, sitemap, or sitemaps"},
			{Name: "id", Help: "the id for that kind"},
		},
	}, buildURL)
}

// newClient builds the client from the host-resolved config, so a host and the
// standalone binary pace, identify, and authenticate themselves the same way.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	return ClientFromConfig(cfg), nil
}

// ClientFromConfig maps the framework config onto a tripadvisor.Config and returns
// a client. The one credential, the Content API key, is read from the environment
// in DefaultConfig, never from a flag.
func ClientFromConfig(cfg kit.Config) *Client {
	tc := DefaultConfig()
	if cfg.Rate > 0 {
		tc.Delay = cfg.Rate
	}
	if cfg.Retries >= 0 {
		tc.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		tc.Timeout = cfg.Timeout
	}
	if ua := cfg.Extra["user-agent"]; ua != "" {
		tc.UserAgent = ua
	} else if cfg.UserAgent != "" {
		tc.UserAgent = cfg.UserAgent
	}
	if v := cfg.Extra["language"]; v != "" {
		tc.Language = v
	}
	if v := cfg.Extra["currency"]; v != "" {
		tc.Currency = v
	}
	if v := cfg.Extra["plane"]; v != "" {
		tc.Plane = v
	}
	if v := cfg.Extra["category"]; v != "" {
		tc.Category = v
	}
	if v := cfg.Extra["lat-long"]; v != "" {
		tc.LatLong = v
	}
	if v := cfg.Extra["radius"]; v != "" {
		tc.Radius = v
	}
	if v := cfg.Extra["radius-unit"]; v != "" {
		tc.RadiusUnit = v
	}
	tc.CacheDir = cfg.CacheDir
	tc.NoCache = cfg.NoCache
	if ttl := cfg.Extra["cache-ttl"]; ttl != "" {
		if d, err := time.ParseDuration(ttl); err == nil {
			tc.CacheTTL = d
		}
	}
	tc.Refresh = cfg.Extra["refresh"] == "true"
	return NewClient(tc)
}

// Defaults seeds the framework baseline with tripadvisor's own values, so an unset
// --rate or --timeout uses the tripadvisor default rather than the generic kit one.
func Defaults(c *kit.Config) {
	def := DefaultConfig()
	c.Rate = def.Delay
	c.Retries = def.Retries
	c.Timeout = def.Timeout
	c.UserAgent = def.UserAgent
}

// Classify turns any accepted input into the canonical (type, id), so `ant resolve`
// and `ant url` touch no network.
func (Domain) Classify(input string) (uriType, id string, err error) {
	r := Classify(input)
	if r.Kind == "unknown" {
		return "", "", errs.Usage("unrecognized tripadvisor reference: %q", input)
	}
	return r.Kind, r.ID, nil
}

// Locate is the inverse: the addressable URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	u := URLFor(uriType, id)
	if u == "" {
		return "", errs.Usage("tripadvisor has no resource type %q", uriType)
	}
	return u, nil
}

// mapErr translates a library error into a kit error so the exit code matches the
// rest of the fleet: a missing entity reads as not found (exit 6), a throttle as
// rate limited (exit 5), the wall or a rejected/missing key as need-auth (exit 4),
// a missing key on an opt-in surface as need-auth, and a caught bad argument as
// usage (exit 2).
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return errs.NotFound("%s", err.Error())
	case errors.Is(err, ErrRateLimited):
		return errs.RateLimited("%s", err.Error())
	case errors.Is(err, ErrBlocked):
		return errs.NeedAuth("%s", err.Error())
	case errors.Is(err, ErrNeedKey):
		return errs.NeedAuth("%s", err.Error())
	case errors.Is(err, ErrUsage):
		return errs.Usage("%s", err.Error())
	default:
		return err
	}
}
