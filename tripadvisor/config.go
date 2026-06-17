package tripadvisor

import (
	"os"
	"time"
)

// config.go holds the resolved settings a Client reads. domain.go's
// ClientFromConfig maps the framework's kit.Config onto this, so the standalone
// binary and a host pace, identify, and authenticate themselves the same way.
//
// There is one credential: the Content API key, read only from the
// TRIPADVISOR_API_KEY environment variable. Its presence turns on the api plane
// for the surfaces the API covers; its absence leaves the tool on the web plane.
// There is no key flag, by design (see the spec section 10.1).

const (
	// Host is the public website, the web plane, and the host the URI driver
	// claims.
	Host = "www.tripadvisor.com"
	// BaseURL is the root of the web plane.
	BaseURL = "https://" + Host
	// ContentURL is the root of the opt-in Content API plane.
	ContentURL = "https://api.content.tripadvisor.com/api/v1"

	// DefaultLanguage is the API language and the web hl parameter.
	DefaultLanguage = "en"
	// DefaultCurrency is the API currency for any price-bearing field.
	DefaultCurrency = "USD"
	// DefaultPlane is the automatic, environment-driven plane choice.
	DefaultPlane = "auto"

	// DefaultCacheTTL is how long a cached response stays fresh by default.
	DefaultCacheTTL = 24 * time.Hour

	defaultLimit = 20 // a bare list command's fetch count
	apiMaxLimit  = 50 // a single Content API page
)

// DefaultUserAgent identifies the client. It names a current browser, because
// the web plane is fronted by DataDome and an obviously scripted agent is turned
// away faster; it is still honest in that the tool does not forge a crawler
// identity it is reverse-DNS-checked against.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

// Config is the resolved settings a Client reads.
type Config struct {
	UserAgent string
	Delay     time.Duration // minimum gap between requests
	Retries   int           // retries on 429/5xx
	Timeout   time.Duration // per-request timeout

	Language string // API language and web hl, e.g. "en"
	Currency string // ISO 4217 currency for price fields, e.g. "USD"

	Plane  string // "web", "api", or "auto"
	APIKey string // the Content API key, from TRIPADVISOR_API_KEY only

	// Search shaping, shared by search and nearby.
	Category   string // hotels, restaurants, attractions, geos
	LatLong    string // "lat,long"
	Radius     string // a number; empty means the API default
	RadiusUnit string // km or mi

	BaseURL    string // overridable for tests
	ContentURL string // overridable for tests

	CacheDir string
	NoCache  bool
	CacheTTL time.Duration
	Refresh  bool // refetch and rewrite the cache, ignoring any hit
}

// DefaultConfig returns the baseline settings and reads the API key from the
// environment.
func DefaultConfig() Config {
	return Config{
		UserAgent:  DefaultUserAgent,
		Delay:      2 * time.Second,
		Retries:    3,
		Timeout:    30 * time.Second,
		Language:   DefaultLanguage,
		Currency:   DefaultCurrency,
		Plane:      DefaultPlane,
		APIKey:     os.Getenv("TRIPADVISOR_API_KEY"),
		BaseURL:    BaseURL,
		ContentURL: ContentURL,
		CacheTTL:   DefaultCacheTTL,
	}
}

// hasKey reports whether an API key is configured.
func (c Config) hasKey() bool { return c.APIKey != "" }
