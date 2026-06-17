package tripadvisor

// types.go holds the exported records the commands emit. Their json tags name the
// fields a reader sees, kit:"id" marks the key the record store upserts on,
// kit:"body" marks the long-text field `ta cat` and the Markdown export print, and
// table:",truncate" keeps wide free text from blowing up a terminal table. Each
// record carries only fields a logged-out web reader or a free Content API key can
// fill: no member accounts, no saved-trip state, no partner nightly rate, no owner
// dashboard metric.
//
// The kit:"link" edges connect the records into one graph a host walks for
// breadth-first crawls. location_id is the universal key, so a review, a photo, a
// search hit, and a sitemap seed all point back at the same location record:
//
//	sitemaps --seeds_ref--> sitemap --(Seed)--> location
//	search   --(Location)--> location          nearby --(Location)--> location
//	location --reviews_ref--> reviews --location--> location
//	location --photos_ref--> photos   --location--> location
//	location --parent_ref--> location (its geo ancestor) --parent_ref--> ... up the geo tree
//	location --nearby_ref--> nearby
//
// No record is a dead leaf: a review and a photo both point back to their location,
// because TripAdvisor exposes no public logged-out member-profile read, so an
// author is a name and an optional location string, not a fabricated user node.
// The attribution fields (URL/web_url, RatingImage/rating_image_url) are always
// kept so a downstream display can comply with the Content API terms.

// Location is the core record: a hotel, restaurant, attraction, or geo. The id is
// the numeric location_id, the same d-number embedded in a page URL and the key the
// Content API uses, so a web read and an API read address the same record.
type Location struct {
	ID           string   `json:"id" kit:"id"` // the numeric location_id, e.g. "188151"
	Name         string   `json:"name,omitempty" table:",truncate"`
	Category     string   `json:"category,omitempty"` // hotel, restaurant, attraction, geo
	Subcategory  []string `json:"subcategory,omitempty" table:"-"`
	Rating       float64  `json:"rating,omitempty"` // 0 to 5
	NumReviews   int      `json:"num_reviews,omitempty"`
	Ranking      string   `json:"ranking,omitempty" table:",truncate"` // "#1 of 1,234 things to do in Paris"
	RankingPos   int      `json:"ranking_pos,omitempty" table:"-"`
	RankingOutOf int      `json:"ranking_out_of,omitempty" table:"-"`
	PriceLevel   string   `json:"price_level,omitempty"`         // "$" .. "$$$$", or "$$ - $$$"
	Cuisine      []string `json:"cuisine,omitempty" table:"-"`   // restaurants
	Amenities    []string `json:"amenities,omitempty" table:"-"` // hotels
	Styles       []string `json:"styles,omitempty" table:"-"`
	TripTypes    []string `json:"trip_types,omitempty" table:"-"`
	Awards       []string `json:"awards,omitempty" table:"-"`
	Phone        string   `json:"phone,omitempty" table:"-"`
	Website      string   `json:"website,omitempty" table:"-"`
	Email        string   `json:"email,omitempty" table:"-"`
	Street       string   `json:"street,omitempty" table:"-"`
	City         string   `json:"city,omitempty"`
	State        string   `json:"state,omitempty" table:"-"`
	Country      string   `json:"country,omitempty" table:"-"`
	Postal       string   `json:"postal,omitempty" table:"-"`
	Address      string   `json:"address,omitempty" table:"-"` // the one-line address_string
	Lat          float64  `json:"lat,omitempty" table:"-"`
	Lng          float64  `json:"lng,omitempty" table:"-"`
	Timezone     string   `json:"timezone,omitempty" table:"-"`
	Hours        []string `json:"hours,omitempty" table:"-"` // weekday_text
	Description  string   `json:"description,omitempty" table:",truncate" kit:"body"`
	Image        string   `json:"image,omitempty" table:",truncate"`
	PhotoCount   int      `json:"photo_count,omitempty" table:"-"`
	RatingImage  string   `json:"rating_image_url,omitempty" table:"-"`                                // attribution duty
	GeoID        string   `json:"geo_id,omitempty" table:"-"`                                          // the g-number of its geo
	URL          string   `json:"url"`                                                                 // web_url, the canonical page
	ReviewsRef   string   `json:"reviews_ref,omitempty" table:"-" kit:"link,kind=tripadvisor/reviews"` // = ID
	PhotosRef    string   `json:"photos_ref,omitempty" table:"-" kit:"link,kind=tripadvisor/photos"`   // = ID
	ParentRef    string   `json:"parent_ref,omitempty" table:"-" kit:"link,kind=tripadvisor/location"` // its geo ancestor
	NearbyRef    string   `json:"nearby_ref,omitempty" table:"-" kit:"link,kind=tripadvisor/nearby"`   // = "lat,lng"
}

// Review is one review of a location, emitted by reviews. Location is the edge back
// to the reviewed place; an author is a name and an optional location, never a
// fabricated member node.
type Review struct {
	ID           string  `json:"id" kit:"id"`
	Location     string  `json:"location,omitempty" table:"-" kit:"link,kind=tripadvisor/location"` // the location_id
	Title        string  `json:"title,omitempty" table:",truncate"`
	Text         string  `json:"text,omitempty" table:",truncate" kit:"body"`
	Rating       float64 `json:"rating,omitempty"`    // 1 to 5
	Published    string  `json:"published,omitempty"` // published_date
	TravelDate   string  `json:"travel_date,omitempty" table:"-"`
	TripType     string  `json:"trip_type,omitempty"` // Family, Couples, Solo, Business, Friends
	Author       string  `json:"author,omitempty"`
	AuthorLoc    string  `json:"author_location,omitempty" table:"-"`
	HelpfulVotes int     `json:"helpful_votes,omitempty" table:"-"`
	Language     string  `json:"language,omitempty" table:"-"`
	OwnerResp    string  `json:"owner_response,omitempty" table:"-"`
	RatingImage  string  `json:"rating_image_url,omitempty" table:"-"`
	URL          string  `json:"url,omitempty" table:"-"`
}

// Photo is one photo of a location, emitted by photos. The size URLs come from the
// Content API images map; Original is the full-size URL.
type Photo struct {
	ID        string `json:"id" kit:"id"`
	Location  string `json:"location,omitempty" table:"-" kit:"link,kind=tripadvisor/location"`
	Caption   string `json:"caption,omitempty" table:",truncate"`
	Published string `json:"published,omitempty"`
	Album     string `json:"album,omitempty" table:"-"`
	Source    string `json:"source,omitempty" table:"-"` // Traveler, Management
	Author    string `json:"author,omitempty" table:"-"`
	Width     int    `json:"width,omitempty" table:"-"` // of the largest size
	Height    int    `json:"height,omitempty" table:"-"`
	Thumbnail string `json:"thumbnail,omitempty" table:"-"`
	Small     string `json:"small,omitempty" table:"-"`
	Medium    string `json:"medium,omitempty" table:"-"`
	Large     string `json:"large,omitempty" table:"-"`
	Original  string `json:"original,omitempty" table:",truncate"` // the full-size URL
}

// Seed is one entry from a TripAdvisor sitemap, emitted by sitemap. The sitemaps
// are the reconstruction backbone: robots.txt advertises a per-section index, each
// index lists shards, and each shard enumerates landing pages. A Seed names one
// page and fills the location edge when Classify resolves a d-number from the URL.
// A page outside the location graph still comes back as a Seed with the URL and
// lastmod and no edge, so no page is dropped.
type Seed struct {
	URL      string `json:"url" kit:"id"`   // the live landing-page URL, the unique key
	Kind     string `json:"kind,omitempty"` // the sitemap kind the seed came from
	ID       string `json:"id,omitempty"`   // the resolved location/geo id, empty if untyped
	Lastmod  string `json:"lastmod,omitempty"`
	Location string `json:"location,omitempty" table:"-" kit:"link,kind=tripadvisor/location"`
}

// SitemapIndex is one published sitemap index, emitted by sitemaps. robots.txt
// advertises every index; this is the root of the backbone. SeedsRef links into
// the sitemap op for that kind, so a crawl walks from here into the seeds.
type SitemapIndex struct {
	URL      string `json:"url" kit:"id"`       // the index URL, the unique key
	Kind     string `json:"kind,omitempty"`     // attractions, attraction_review, location_photo, show_user_reviews, business
	Category string `json:"category,omitempty"` // attraction, review, photo, business, place, other
	SeedsRef string `json:"seeds_ref,omitempty" table:"-" kit:"link,kind=tripadvisor/sitemap"`
}

// Ref is the result of `ta ref id`: the canonical (kind, id) a reference resolves
// to, plus the URL, all without touching the network.
type Ref struct {
	Input string `json:"input"`
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	URL   string `json:"url"`
}
