package tripadvisor

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
)

// api.go reads the Content API plane (api.content.tripadvisor.com). It decodes the
// documented envelopes: the {data:[...]} list for search/nearby/reviews/photos and
// the bare object for details. Numbers arrive as JSON strings in this API
// ("rating":"4.5", "num_reviews":"140000"), so the decoders use fnum and never
// assume a JSON number. Every request carries the key from the environment plus the
// language and currency; the key never lands in a log or a cache filename.

// apiURL builds a Content API URL with the key, language, and currency folded in.
func (c *Client) apiURL(path string, q url.Values) string {
	if q == nil {
		q = url.Values{}
	}
	q.Set("key", c.cfg.APIKey)
	if c.cfg.Language != "" {
		q.Set("language", c.cfg.Language)
	}
	if c.cfg.Currency != "" {
		q.Set("currency", c.cfg.Currency)
	}
	return strings.TrimRight(c.cfg.ContentURL, "/") + path + "?" + q.Encode()
}

// --- envelopes ---

type apiList struct {
	Data []apiLocation `json:"data"`
}

type apiAddress struct {
	Street1       string `json:"street1"`
	Street2       string `json:"street2"`
	City          string `json:"city"`
	State         string `json:"state"`
	Country       string `json:"country"`
	PostalCode    string `json:"postalcode"`
	AddressString string `json:"address_string"`
}

type apiNamed struct {
	Name          string `json:"name"`
	LocalizedName string `json:"localized_name"`
}

func (n apiNamed) text() string {
	if n.LocalizedName != "" {
		return n.LocalizedName
	}
	return n.Name
}

type apiAncestor struct {
	Level              string `json:"level"`
	Name               string `json:"name"`
	LocationID         string `json:"location_id"`
	AbbreviatedAddress string `json:"abbrv_addr_string"`
}

type apiRanking struct {
	GeoLocationID   string `json:"geo_location_id"`
	RankingString   string `json:"ranking_string"`
	GeoLocationName string `json:"geo_location_name"`
	RankingOutOf    fnum   `json:"ranking_out_of"`
	Ranking         fnum   `json:"ranking"`
}

type apiHours struct {
	WeekdayText []string `json:"weekday_text"`
}

type apiLocation struct {
	LocationID  string        `json:"location_id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	WebURL      string        `json:"web_url"`
	Address     apiAddress    `json:"address_obj"`
	Ancestors   []apiAncestor `json:"ancestors"`
	Latitude    fnum          `json:"latitude"`
	Longitude   fnum          `json:"longitude"`
	Timezone    string        `json:"timezone"`
	Phone       string        `json:"phone"`
	Website     string        `json:"website"`
	Email       string        `json:"email"`
	Rating      fnum          `json:"rating"`
	NumReviews  fnum          `json:"num_reviews"`
	RatingImage string        `json:"rating_image_url"`
	PriceLevel  string        `json:"price_level"`
	Ranking     apiRanking    `json:"ranking_data"`
	Hours       apiHours      `json:"hours"`
	Category    apiNamed      `json:"category"`
	Subcategory []apiNamed    `json:"subcategory"`
	Groups      []apiGroup    `json:"groups"`
	Cuisine     []apiNamed    `json:"cuisine"`
	TripTypes   []apiNamed    `json:"trip_types"`
	Styles      []string      `json:"styles"`
	Amenities   []string      `json:"amenities"`
	Awards      []apiAward    `json:"awards"`
	PhotoCount  fnum          `json:"photo_count"`
}

type apiGroup struct {
	Name       string     `json:"name"`
	Categories []apiNamed `json:"categories"`
}

type apiAward struct {
	AwardType   string `json:"award_type"`
	Year        string `json:"year"`
	DisplayName string `json:"display_name"`
}

// toLocation maps an API location envelope onto the shared record.
func (a apiLocation) toLocation() *Location {
	loc := &Location{
		ID:           a.LocationID,
		Name:         squish(a.Name),
		Category:     normalizeCategory(a.Category.text()),
		Rating:       a.Rating.float(),
		NumReviews:   a.NumReviews.int(),
		Ranking:      squish(a.Ranking.RankingString),
		RankingPos:   a.Ranking.Ranking.int(),
		RankingOutOf: a.Ranking.RankingOutOf.int(),
		PriceLevel:   strings.TrimSpace(a.PriceLevel),
		Cuisine:      namedList(a.Cuisine),
		Amenities:    trimAll(a.Amenities),
		Styles:       trimAll(a.Styles),
		TripTypes:    namedList(a.TripTypes),
		Awards:       awardList(a.Awards),
		Phone:        strings.TrimSpace(a.Phone),
		Website:      strings.TrimSpace(a.Website),
		Email:        strings.TrimSpace(a.Email),
		Street:       squish(joinStreet(a.Address)),
		City:         squish(a.Address.City),
		State:        squish(a.Address.State),
		Country:      squish(a.Address.Country),
		Postal:       strings.TrimSpace(a.Address.PostalCode),
		Address:      squish(a.Address.AddressString),
		Lat:          a.Latitude.float(),
		Lng:          a.Longitude.float(),
		Timezone:     strings.TrimSpace(a.Timezone),
		Hours:        trimAll(a.Hours.WeekdayText),
		Description:  squish(a.Description),
		PhotoCount:   a.PhotoCount.int(),
		RatingImage:  strings.TrimSpace(a.RatingImage),
		URL:          strings.TrimSpace(a.WebURL),
	}
	for _, s := range a.Subcategory {
		if v := s.text(); v != "" {
			loc.Subcategory = append(loc.Subcategory, v)
		}
	}
	loc.GeoID = geoAncestor(a.Ancestors)
	wireLocationEdges(loc)
	return loc
}

// geoAncestor returns the nearest geo ancestor's location id, the parent geo node.
func geoAncestor(ancestors []apiAncestor) string {
	for _, anc := range ancestors {
		if anc.LocationID != "" {
			return anc.LocationID
		}
	}
	return ""
}

// normalizeCategory folds the API category name to ta's vocabulary.
func normalizeCategory(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "hotel", "hotels", "lodging", "accommodation":
		return "hotel"
	case "restaurant", "restaurants":
		return "restaurant"
	case "attraction", "attractions", "things to do":
		return "attraction"
	case "geo", "geos", "geographic":
		return "geo"
	default:
		return strings.ToLower(strings.TrimSpace(name))
	}
}

func joinStreet(a apiAddress) string {
	parts := []string{a.Street1, a.Street2}
	var keep []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			keep = append(keep, strings.TrimSpace(p))
		}
	}
	return strings.Join(keep, " ")
}

func namedList(in []apiNamed) []string {
	var out []string
	for _, n := range in {
		if v := squish(n.text()); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func awardList(in []apiAward) []string {
	var out []string
	for _, a := range in {
		name := strings.TrimSpace(a.DisplayName)
		if name == "" {
			continue
		}
		if a.Year != "" {
			name += " (" + a.Year + ")"
		}
		out = append(out, name)
	}
	return out
}

// --- reviews ---

type apiReviewList struct {
	Data []apiReview `json:"data"`
}

type apiReview struct {
	ID            json.Number `json:"id"`
	LocationID    string      `json:"location_id"`
	Lang          string      `json:"lang"`
	PublishedDate string      `json:"published_date"`
	Rating        fnum        `json:"rating"`
	HelpfulVotes  fnum        `json:"helpful_votes"`
	RatingImage   string      `json:"rating_image_url"`
	URL           string      `json:"url"`
	TripType      string      `json:"trip_type"`
	TravelDate    string      `json:"travel_date"`
	Text          string      `json:"text"`
	Title         string      `json:"title"`
	OwnerResp     *apiOwner   `json:"owner_response"`
	User          apiUser     `json:"user"`
}

type apiOwner struct {
	Text string `json:"text"`
}

type apiUser struct {
	Username     string          `json:"username"`
	UserLocation apiUserLocation `json:"user_location"`
}

type apiUserLocation struct {
	Name string `json:"name"`
}

func (a apiReview) toReview(loc string) *Review {
	id := strings.TrimSpace(a.ID.String())
	location := firstNonEmpty(a.LocationID, loc)
	r := &Review{
		ID:           id,
		Location:     location,
		Title:        squish(a.Title),
		Text:         squish(a.Text),
		Rating:       a.Rating.float(),
		Published:    strings.TrimSpace(a.PublishedDate),
		TravelDate:   strings.TrimSpace(a.TravelDate),
		TripType:     squish(a.TripType),
		Author:       squish(a.User.Username),
		AuthorLoc:    squish(a.User.UserLocation.Name),
		HelpfulVotes: a.HelpfulVotes.int(),
		Language:     strings.TrimSpace(a.Lang),
		RatingImage:  strings.TrimSpace(a.RatingImage),
		URL:          strings.TrimSpace(a.URL),
	}
	if a.OwnerResp != nil {
		r.OwnerResp = squish(a.OwnerResp.Text)
	}
	return r
}

// --- photos ---

type apiPhotoList struct {
	Data []apiPhoto `json:"data"`
}

type apiPhoto struct {
	ID            json.Number `json:"id"`
	Caption       string      `json:"caption"`
	PublishedDate string      `json:"published_date"`
	Album         string      `json:"album"`
	Source        apiNamed    `json:"source"`
	User          apiUser     `json:"user"`
	Images        apiImageSet `json:"images"`
}

type apiImageSet struct {
	Thumbnail apiImage `json:"thumbnail"`
	Small     apiImage `json:"small"`
	Medium    apiImage `json:"medium"`
	Large     apiImage `json:"large"`
	Original  apiImage `json:"original"`
}

type apiImage struct {
	URL    string `json:"url"`
	Width  fnum   `json:"width"`
	Height fnum   `json:"height"`
}

func (a apiPhoto) toPhoto(loc string) *Photo {
	p := &Photo{
		ID:        strings.TrimSpace(a.ID.String()),
		Location:  loc,
		Caption:   squish(a.Caption),
		Published: strings.TrimSpace(a.PublishedDate),
		Album:     squish(a.Album),
		Source:    squish(a.Source.text()),
		Author:    squish(a.User.Username),
		Thumbnail: a.Images.Thumbnail.URL,
		Small:     a.Images.Small.URL,
		Medium:    a.Images.Medium.URL,
		Large:     a.Images.Large.URL,
		Original:  a.Images.Original.URL,
	}
	// The largest present size fills Width/Height.
	for _, img := range []apiImage{a.Images.Original, a.Images.Large, a.Images.Medium, a.Images.Small, a.Images.Thumbnail} {
		if img.Width.int() > 0 {
			p.Width = img.Width.int()
			p.Height = img.Height.int()
			break
		}
	}
	return p
}

// --- Client methods ---

// SearchAPI finds locations by name on the Content API.
func (c *Client) SearchAPI(ctx context.Context, query string, limit int) ([]*Location, error) {
	q := url.Values{}
	q.Set("searchQuery", query)
	if c.cfg.Category != "" {
		q.Set("category", c.cfg.Category)
	}
	if c.cfg.LatLong != "" {
		q.Set("latLong", c.cfg.LatLong)
	}
	if c.cfg.Radius != "" {
		q.Set("radius", c.cfg.Radius)
		if c.cfg.RadiusUnit != "" {
			q.Set("radiusUnit", c.cfg.RadiusUnit)
		}
	}
	return c.locationList(ctx, "/location/search", q, limit)
}

// NearbyAPI returns locations near a "lat,long" point.
func (c *Client) NearbyAPI(ctx context.Context, latLong string, limit int) ([]*Location, error) {
	q := url.Values{}
	q.Set("latLong", latLong)
	if c.cfg.Category != "" {
		q.Set("category", c.cfg.Category)
	}
	if c.cfg.Radius != "" {
		q.Set("radius", c.cfg.Radius)
		if c.cfg.RadiusUnit != "" {
			q.Set("radiusUnit", c.cfg.RadiusUnit)
		}
	}
	return c.locationList(ctx, "/location/nearby_search", q, limit)
}

func (c *Client) locationList(ctx context.Context, path string, q url.Values, limit int) ([]*Location, error) {
	body, err := c.getAPI(ctx, c.apiURL(path, q))
	if err != nil {
		return nil, err
	}
	var env apiList
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, ErrNotFound
	}
	var out []*Location
	for _, l := range env.Data {
		out = append(out, l.toLocation())
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// DetailsAPI returns one location in full.
func (c *Client) DetailsAPI(ctx context.Context, id string) (*Location, error) {
	body, err := c.getAPI(ctx, c.apiURL("/location/"+url.PathEscape(id)+"/details", nil))
	if err != nil {
		return nil, err
	}
	var a apiLocation
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, ErrNotFound
	}
	if a.LocationID == "" {
		a.LocationID = id
	}
	return a.toLocation(), nil
}

// ReviewsAPI returns a location's reviews, paginated by limit.
func (c *Client) ReviewsAPI(ctx context.Context, id string, limit int) ([]*Review, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(min(limit, apiMaxLimit)))
	}
	body, err := c.getAPI(ctx, c.apiURL("/location/"+url.PathEscape(id)+"/reviews", q))
	if err != nil {
		return nil, err
	}
	var env apiReviewList
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, ErrNotFound
	}
	var out []*Review
	for _, r := range env.Data {
		out = append(out, r.toReview(id))
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

// PhotosAPI returns a location's photos, paginated by limit.
func (c *Client) PhotosAPI(ctx context.Context, id string, limit int) ([]*Photo, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(min(limit, apiMaxLimit)))
	}
	body, err := c.getAPI(ctx, c.apiURL("/location/"+url.PathEscape(id)+"/photos", q))
	if err != nil {
		return nil, err
	}
	var env apiPhotoList
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, ErrNotFound
	}
	var out []*Photo
	for _, p := range env.Data {
		out = append(out, p.toPhoto(id))
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
