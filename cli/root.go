// Package cli assembles the ta command tree from the tripadvisor domain on top of
// the any-cli/kit framework.
package cli

import (
	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/tripadvisor-cli/tripadvisor"
)

// Build metadata, set via -ldflags at release time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// builder holds the domain-global flags while the app is assembled, then folds them
// onto the resolved config in finalize, using the exact keys ClientFromConfig
// reads. There is no key flag: the Content API key is read only from
// TRIPADVISOR_API_KEY in the environment.
type builder struct {
	userAgent  string
	language   string
	currency   string
	plane      string
	category   string
	latLong    string
	radius     string
	radiusUnit string
	cacheTTL   string
	refresh    bool
}

// NewApp assembles the kit application from the tripadvisor domain. The domain's
// Register installs the client factory and every operation, so the binary and a
// host (ant, which blank-imports the package) share one source of truth. This
// package adds the domain-global flags and the version command; kit.Run turns the
// App into the CLI, plus the serve and mcp surfaces and the typed-error-to-exit-
// code mapping.
//
// To add a command, declare it in tripadvisor/domain.go with kit.Handle and it
// appears here automatically. Reach for app.AddCommand only for a verb that does
// not fit the emit-records shape, the way version does below.
func NewApp() *kit.App {
	b := &builder{}
	id := tripadvisor.Identity()
	id.Version = Version

	app := kit.New(id, kit.WithDefaults(tripadvisor.Defaults))
	app.GlobalFlags(b.globals)
	app.Finalize(b.finalize)

	tripadvisor.Domain{}.Register(app)
	app.AddCommand(newVersionCmd())
	return app
}

func (b *builder) globals(f *kit.FlagSet) {
	def := tripadvisor.DefaultConfig()
	f.StringVar(&b.userAgent, "user-agent", tripadvisor.DefaultUserAgent, "User-Agent sent with each request")
	f.StringVar(&b.language, "language", def.Language, "language for the API and the web hl")
	f.StringVar(&b.currency, "currency", def.Currency, "ISO 4217 currency for price fields")
	f.StringVar(&b.plane, "plane", def.Plane, "which plane to read: web, api, or auto")
	f.StringVar(&b.category, "category", "", "search category: hotels, restaurants, attractions, or geos")
	f.StringVar(&b.latLong, "lat-long", "", "bias search to a point as \"lat,long\"")
	f.StringVar(&b.radius, "radius", "", "search radius around lat-long")
	f.StringVar(&b.radiusUnit, "radius-unit", "", "radius unit: km or mi")
	f.StringVar(&b.cacheTTL, "cache-ttl", tripadvisor.DefaultCacheTTL.String(), "how long a cached response stays fresh")
	f.BoolVar(&b.refresh, "refresh", false, "fetch fresh copies and rewrite the cache, ignoring any hit")
}

func (b *builder) finalize(c *kit.Config) {
	if c.Extra == nil {
		c.Extra = map[string]string{}
	}
	set := func(k, v string) {
		if v != "" {
			c.Extra[k] = v
		}
	}
	set("user-agent", b.userAgent)
	set("language", b.language)
	set("currency", b.currency)
	set("plane", b.plane)
	set("category", b.category)
	set("lat-long", b.latLong)
	set("radius", b.radius)
	set("radius-unit", b.radiusUnit)
	set("cache-ttl", b.cacheTTL)
	if b.refresh {
		c.Extra["refresh"] = "true"
	}
}
