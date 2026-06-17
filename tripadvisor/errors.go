package tripadvisor

import "errors"

// Sentinel errors the library returns; domain.go's mapErr turns each into the
// kit error kind that carries the right exit code (see the spec section 4.5).
var (
	// ErrBlocked is the wall: a DataDome 403 or challenge interstitial on the web
	// plane, or a rejected/missing key (401) on the api plane. It maps to need-auth
	// (exit 4), with a remedy of reading from a residential or mobile connection or
	// setting a free TRIPADVISOR_API_KEY.
	ErrBlocked = errors.New("blocked: TripAdvisor's web plane is walled by DataDome here. Read from a residential or mobile connection, or set a free TRIPADVISOR_API_KEY for the Content API plane")

	// ErrNeedKey is the api-only surface reached with no key. Maps to exit 4.
	ErrNeedKey = errors.New("this surface needs the Content API: set TRIPADVISOR_API_KEY (a free key from https://www.tripadvisor.com/developers)")

	// ErrNotFound is a missing location (a 404 or not-found envelope). Maps to exit 6.
	ErrNotFound = errors.New("not found")

	// ErrRateLimited is a sustained 429 after retries, or the free-key cap. Maps to exit 5.
	ErrRateLimited = errors.New("rate limited by TripAdvisor: slow down with --rate or try again later")

	// ErrUsage is a bad argument the library catches (an unrecognized reference,
	// a bad category). Maps to exit 2.
	ErrUsage = errors.New("usage")
)
