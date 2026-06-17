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

// ldNodes returns every JSON-LD node in body, expanding an @graph wrapper so a
// node nested in a @graph array is reached the same way as a top-level island.
// TripAdvisor serves the place island sometimes bare and sometimes inside a
// @graph alongside the BreadcrumbList, so both shapes parse through one path.
func ldNodes(body []byte) [][]byte {
	var out [][]byte
	for _, block := range ldBlocks(body) {
		var probe struct {
			Graph json.RawMessage `json:"@graph"`
		}
		if err := json.Unmarshal(block, &probe); err == nil && len(probe.Graph) > 0 {
			var nodes []json.RawMessage
			if err := json.Unmarshal(probe.Graph, &nodes); err == nil {
				for _, n := range nodes {
					out = append(out, bytes.TrimSpace(n))
				}
				continue
			}
		}
		out = append(out, block)
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
	for _, block := range ldNodes(body) {
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
		// The island carries no geo chain, but the page's BreadcrumbList does, so a
		// keyless web read still reconstructs the lineage the API would have given.
		loc.Ancestors = ancestorsFromLD(body, id)
		wireLocationEdges(loc)
		return loc
	}
	return nil
}

// ldBreadcrumb is a schema.org BreadcrumbList, the page's geo trail.
type ldBreadcrumb struct {
	Type  jsonType  `json:"@type"`
	Items []ldCrumb `json:"itemListElement"`
}

type ldCrumb struct {
	Position int    `json:"position"`
	Name     string `json:"name"`
	Item     ldItem `json:"item"`
}

// ldItem decodes a breadcrumb item that is either a bare URL string or an object
// carrying @id (or url) and an optional name.
type ldItem struct {
	URL  string
	Name string
}

func (i *ldItem) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	if b[0] == '"' {
		return json.Unmarshal(b, &i.URL)
	}
	var obj struct {
		ID   string `json:"@id"`
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return nil
	}
	i.URL = firstNonEmpty(obj.ID, obj.URL)
	i.Name = obj.Name
	return nil
}

// ancestorsFromLD reads a BreadcrumbList island into the geo lineage, nearest
// first. The trail runs top-down (country to the place itself); the place's own
// crumb (the one resolving back to locID) and any crumb that is not a geo are
// dropped, duplicates are removed, and the rest are reversed so the nearest geo is
// first, matching the API's ancestor order.
func ancestorsFromLD(body []byte, locID string) []Ancestor {
	for _, node := range ldNodes(body) {
		var bc ldBreadcrumb
		if err := json.Unmarshal(node, &bc); err != nil {
			continue
		}
		if string(bc.Type) != "BreadcrumbList" || len(bc.Items) == 0 {
			continue
		}
		var chain []Ancestor
		seen := map[string]bool{}
		for _, it := range bc.Items {
			if it.Item.URL == "" {
				continue
			}
			r := Classify(it.Item.URL)
			if r.Kind != "location" || r.ID == "" || r.ID == locID || seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			chain = append(chain, Ancestor{ID: r.ID, Name: squish(firstNonEmpty(it.Item.Name, it.Name))})
		}
		for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
			chain[i], chain[j] = chain[j], chain[i]
		}
		if len(chain) > 0 {
			return chain
		}
	}
	return nil
}

// imagesFromLD returns every image URL the first place island carries, so the web
// photos fallback surfaces all island images, not only the hero.
func imagesFromLD(body []byte) []string {
	for _, node := range ldNodes(body) {
		var d ldDoc
		if err := json.Unmarshal(node, &d); err != nil {
			continue
		}
		if categoryFor(string(d.Type)) == "" {
			continue
		}
		return trimAll(d.Image)
	}
	return nil
}

// reviewsFromLD lifts the embedded review array of the first place island, the
// recent-reviews subset the web plane carries. id is the reviewed location.
func reviewsFromLD(body []byte, id string) []*Review {
	for _, block := range ldNodes(body) {
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

// wireLocationEdges fills the graph edges that derive from a location's own id and
// its geo lineage. The reviews/photos edges are the location id; the parent is the
// nearest geo; ancestor_refs and neighborhood_refs are the up and down geo edges a
// crawl follows to reach the whole hierarchy; nearby is the coordinate.
func wireLocationEdges(loc *Location) {
	if loc.ID == "" {
		return
	}
	loc.ReviewsRef = loc.ID
	loc.PhotosRef = loc.ID
	if loc.GeoID != "" && loc.GeoID != loc.ID {
		loc.ParentRef = loc.GeoID
	}
	loc.AncestorRefs = loc.AncestorRefs[:0]
	for _, a := range loc.Ancestors {
		if a.ID != "" && a.ID != loc.ID {
			loc.AncestorRefs = append(loc.AncestorRefs, a.ID)
		}
	}
	if len(loc.AncestorRefs) == 0 {
		loc.AncestorRefs = nil
	}
	loc.NeighborhoodRefs = loc.NeighborhoodRefs[:0]
	for _, n := range loc.Neighborhoods {
		if n.ID != "" && n.ID != loc.ID {
			loc.NeighborhoodRefs = append(loc.NeighborhoodRefs, n.ID)
		}
	}
	if len(loc.NeighborhoodRefs) == 0 {
		loc.NeighborhoodRefs = nil
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
