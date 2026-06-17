package tripadvisor

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// jsonld.go extracts and decodes the schema.org JSON-LD islands a TripAdvisor
// location page embeds. The island is the web plane's source for the location and
// (in part) the reviews commands. Field types are decoded flexibly because
// schema.org plays loose with JSON: a field that is sometimes a string is sometimes
// an object or array, and a number is sometimes quoted.
//
// The location id and geo id are not in the island; they come from the page URL via
// Classify, so the parse functions take them as arguments.

var ldScriptRE = regexp.MustCompile(`(?is)<script[^>]+type="application/ld\+json"[^>]*>(.*?)</script>`)

// ldBlocks returns the raw JSON of every ld+json script in an HTML body.
func ldBlocks(body []byte) [][]byte {
	var out [][]byte
	for _, m := range ldScriptRE.FindAllSubmatch(body, -1) {
		out = append(out, bytes.TrimSpace(m[1]))
	}
	return out
}

// ldDoc is the subset of a schema.org place island ta reads.
type ldDoc struct {
	Type            jsonType   `json:"@type"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	URL             string     `json:"url"`
	Image           jsonStr    `json:"image"`
	PriceRange      string     `json:"priceRange"`
	Telephone       string     `json:"telephone"`
	ServesCuisine   jsonStr    `json:"servesCuisine"`
	Address         ldAddress  `json:"address"`
	Geo             ldGeo      `json:"geo"`
	AggregateRating ldAgg      `json:"aggregateRating"`
	Review          []ldReview `json:"review"`
}

type ldAddress struct {
	StreetAddress   string    `json:"streetAddress"`
	AddressLocality string    `json:"addressLocality"`
	AddressRegion   string    `json:"addressRegion"`
	PostalCode      string    `json:"postalCode"`
	AddressCountry  ldCountry `json:"addressCountry"`
}

// ldCountry decodes addressCountry that is either a bare string or a {name} object.
type ldCountry struct {
	Name string
}

func (c *ldCountry) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	if b[0] == '"' {
		return json.Unmarshal(b, &c.Name)
	}
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil
	}
	c.Name = obj.Name
	return nil
}

type ldGeo struct {
	Latitude  fnum `json:"latitude"`
	Longitude fnum `json:"longitude"`
}

type ldAgg struct {
	RatingValue fnum `json:"ratingValue"`
	ReviewCount fnum `json:"reviewCount"`
	BestRating  fnum `json:"bestRating"`
}

type ldRating struct {
	RatingValue fnum `json:"ratingValue"`
}

type ldReview struct {
	Author        ldAuthor `json:"author"`
	ReviewBody    string   `json:"reviewBody"`
	Name          string   `json:"name"`
	DatePublished string   `json:"datePublished"`
	InLanguage    string   `json:"inLanguage"`
	ReviewRating  ldRating `json:"reviewRating"`
}

// ldAuthor decodes an author that is either a bare string or a {name} object.
type ldAuthor struct {
	Name string
}

func (a *ldAuthor) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	if b[0] == '"' {
		return json.Unmarshal(b, &a.Name)
	}
	var obj struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil
	}
	a.Name = obj.Name
	return nil
}

// categoryFor maps a schema.org @type to a ta category, or "" when the type is not
// a place island ta models.
func categoryFor(t string) string {
	switch t {
	case "Hotel", "LodgingBusiness", "Resort", "BedAndBreakfast", "Motel":
		return "hotel"
	case "Restaurant", "FoodEstablishment", "CafeOrCoffeeShop", "BarOrPub":
		return "restaurant"
	case "TouristAttraction", "Attraction", "Museum", "Park", "LandmarksOrHistoricalBuildings":
		return "attraction"
	case "LocalBusiness", "Place":
		return "attraction"
	default:
		return ""
	}
}

// locationFromLD builds a Location from the first place island in body, with the id
// and geo supplied by the caller (they come from the page URL, not the island). It
// returns nil when no block is a place island.
func locationFromLD(body []byte, id, geo, webURL string) *Location {
	for _, block := range ldBlocks(body) {
		var d ldDoc
		if err := json.Unmarshal(block, &d); err != nil {
			continue // one malformed block must not sink the rest
		}
		cat := categoryFor(string(d.Type))
		if cat == "" {
			continue
		}
		loc := &Location{
			ID:          id,
			Name:        squish(d.Name),
			Category:    cat,
			Rating:      d.AggregateRating.RatingValue.float(),
			NumReviews:  d.AggregateRating.ReviewCount.int(),
			PriceLevel:  strings.TrimSpace(d.PriceRange),
			Cuisine:     trimAll(d.ServesCuisine),
			Phone:       strings.TrimSpace(d.Telephone),
			Street:      squish(d.Address.StreetAddress),
			City:        squish(d.Address.AddressLocality),
			State:       squish(d.Address.AddressRegion),
			Country:     squish(d.Address.AddressCountry.Name),
			Postal:      strings.TrimSpace(d.Address.PostalCode),
			Lat:         d.Geo.Latitude.float(),
			Lng:         d.Geo.Longitude.float(),
			Description: squish(d.Description),
			GeoID:       geo,
			URL:         firstNonEmpty(squish(d.URL), webURL),
		}
		if len(d.Image) > 0 {
			loc.Image = d.Image[0]
		}
		wireLocationEdges(loc)
		return loc
	}
	return nil
}

// reviewsFromLD lifts the embedded review array of the first place island, the
// recent-reviews subset the web plane carries. id is the reviewed location.
func reviewsFromLD(body []byte, id string) []*Review {
	for _, block := range ldBlocks(body) {
		var d ldDoc
		if err := json.Unmarshal(block, &d); err != nil {
			continue
		}
		if categoryFor(string(d.Type)) == "" || len(d.Review) == 0 {
			continue
		}
		var out []*Review
		for i, rv := range d.Review {
			out = append(out, &Review{
				ID:        id + "-ld-" + strconv.Itoa(i),
				Location:  id,
				Title:     squish(rv.Name),
				Text:      squish(rv.ReviewBody),
				Rating:    rv.ReviewRating.RatingValue.float(),
				Published: strings.TrimSpace(rv.DatePublished),
				Author:    squish(rv.Author.Name),
				Language:  strings.TrimSpace(rv.InLanguage),
			})
		}
		return out
	}
	return nil
}

// wireLocationEdges fills the graph edges that derive from a location's own id.
func wireLocationEdges(loc *Location) {
	if loc.ID == "" {
		return
	}
	loc.ReviewsRef = loc.ID
	loc.PhotosRef = loc.ID
	if loc.GeoID != "" && loc.GeoID != loc.ID {
		loc.ParentRef = loc.GeoID
	}
	if loc.Lat != 0 || loc.Lng != 0 {
		loc.NearbyRef = strconv.FormatFloat(loc.Lat, 'f', -1, 64) + "," +
			strconv.FormatFloat(loc.Lng, 'f', -1, 64)
	}
}

// --- flexible JSON decoders, shared with api.go where the API quotes its numbers ---

// jsonType decodes @type whether it is a string or an array of strings, keeping the
// first.
type jsonType string

func (t *jsonType) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	if b[0] == '[' {
		var arr []string
		if err := json.Unmarshal(b, &arr); err != nil {
			return nil
		}
		if len(arr) > 0 {
			*t = jsonType(arr[0])
		}
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return nil
	}
	*t = jsonType(s)
	return nil
}

// jsonStr decodes a field that is either a string or an array of strings into a
// slice.
type jsonStr []string

func (j *jsonStr) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	if b[0] == '[' {
		var arr []string
		if err := json.Unmarshal(b, &arr); err != nil {
			return nil
		}
		*j = arr
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return nil
	}
	if s != "" {
		*j = []string{s}
	}
	return nil
}

// fnum decodes a number that may be quoted, e.g. 4.5 or "4.5". The Content API
// quotes all of its numbers, so this is shared with api.go.
type fnum float64

func (f *fnum) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(bytes.TrimSpace(b), `"`)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	v, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return nil
	}
	*f = fnum(v)
	return nil
}

func (f fnum) float() float64 { return float64(f) }
func (f fnum) int() int       { return int(float64(f)) }

// trimAll squishes each string in a slice and drops the empties.
func trimAll(in []string) []string {
	var out []string
	for _, s := range in {
		if v := squish(s); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// firstNonEmpty returns the first non-empty argument.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
