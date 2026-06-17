package tripadvisor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// hotelLD is a captured-shape JSON-LD island for a hotel page, with the loose field
// types schema.org allows (a number quoted, an author as an object).
const hotelLD = `<html><head>
<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Hotel","name":"The Plaza",
 "description":"A landmark hotel on Fifth Avenue.","priceRange":"$$$$",
 "telephone":"+1 212-759-3000",
 "address":{"@type":"PostalAddress","streetAddress":"768 5th Ave","addressLocality":"New York City","addressRegion":"NY","postalCode":"10019","addressCountry":{"@type":"Country","name":"United States"}},
 "geo":{"@type":"GeoCoordinates","latitude":"40.7644","longitude":"-73.9742"},
 "aggregateRating":{"@type":"AggregateRating","ratingValue":"4.5","reviewCount":"5436"},
 "image":"https://media.tacdn.com/plaza.jpg",
 "review":[{"@type":"Review","author":{"@type":"Person","name":"Jane T"},"name":"Wonderful stay","reviewBody":"Beautiful rooms and great service.","datePublished":"2026-05-01","inLanguage":"en","reviewRating":{"@type":"Rating","ratingValue":5}}]}
</script>
</head></html>`

func TestLocationFromLD(t *testing.T) {
	loc := locationFromLD([]byte(hotelLD), "93450", "60763", "https://www.tripadvisor.com/Hotel_Review-g60763-d93450.html")
	if loc == nil {
		t.Fatal("locationFromLD returned nil")
	}
	if loc.ID != "93450" || loc.GeoID != "60763" {
		t.Errorf("id/geo = %q/%q", loc.ID, loc.GeoID)
	}
	if loc.Name != "The Plaza" || loc.Category != "hotel" {
		t.Errorf("name/category = %q/%q", loc.Name, loc.Category)
	}
	if loc.Rating != 4.5 || loc.NumReviews != 5436 {
		t.Errorf("rating/reviews = %v/%v", loc.Rating, loc.NumReviews)
	}
	if loc.City != "New York City" || loc.Country != "United States" {
		t.Errorf("city/country = %q/%q", loc.City, loc.Country)
	}
	if loc.Lat == 0 || loc.Lng == 0 {
		t.Errorf("geo not parsed: %v,%v", loc.Lat, loc.Lng)
	}
	if loc.ReviewsRef != "93450" || loc.PhotosRef != "93450" || loc.ParentRef != "60763" {
		t.Errorf("edges = %q/%q/%q", loc.ReviewsRef, loc.PhotosRef, loc.ParentRef)
	}
}

func TestReviewsFromLD(t *testing.T) {
	rs := reviewsFromLD([]byte(hotelLD), "93450")
	if len(rs) != 1 {
		t.Fatalf("got %d reviews, want 1", len(rs))
	}
	r := rs[0]
	if r.Location != "93450" || r.Author != "Jane T" || r.Rating != 5 {
		t.Errorf("review = %+v", r)
	}
	if !strings.Contains(r.Text, "Beautiful rooms") {
		t.Errorf("text = %q", r.Text)
	}
}

// apiDetails is a captured-shape Content API details object, with the API's quoted
// numbers and address_obj/ancestors/ranking_data/hours.
const apiDetails = `{
 "location_id":"188757","name":"Eiffel Tower","description":"Paris landmark.",
 "web_url":"https://www.tripadvisor.com/Attraction_Review-g187147-d188757.html",
 "address_obj":{"street1":"Champ de Mars","city":"Paris","country":"France","postalcode":"75007","address_string":"Champ de Mars, 5 Avenue Anatole France, 75007 Paris France"},
 "ancestors":[{"level":"City","name":"Paris","location_id":"187147"},{"level":"Country","name":"France","location_id":"187070"}],
 "latitude":"48.8584","longitude":"2.2945","timezone":"Europe/Paris",
 "rating":"4.5","num_reviews":"140126","rating_image_url":"https://www.tripadvisor.com/img/cdsi/img2/ratings/4.5.svg",
 "ranking_data":{"ranking_string":"#1 of 3,236 things to do in Paris","ranking":"1","ranking_out_of":"3236"},
 "category":{"name":"attraction","localized_name":"Attraction"},
 "subcategory":[{"name":"sights","localized_name":"Sights & Landmarks"}],
 "hours":{"weekday_text":["Monday: 9:00 AM - 12:45 AM"]},
 "photo_count":"50000"}`

func TestAPIDetails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") == "" {
			t.Error("API request carried no key")
		}
		_, _ = w.Write([]byte(apiDetails))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	c.cfg.APIKey = "TESTKEY"
	loc, err := c.DetailsAPI(context.Background(), "188757")
	if err != nil {
		t.Fatal(err)
	}
	if loc.Name != "Eiffel Tower" || loc.Category != "attraction" {
		t.Errorf("name/category = %q/%q", loc.Name, loc.Category)
	}
	if loc.Rating != 4.5 || loc.NumReviews != 140126 {
		t.Errorf("rating/reviews = %v/%v", loc.Rating, loc.NumReviews)
	}
	if loc.RankingPos != 1 || loc.RankingOutOf != 3236 {
		t.Errorf("ranking = %d/%d", loc.RankingPos, loc.RankingOutOf)
	}
	if loc.GeoID != "187147" || loc.ParentRef != "187147" {
		t.Errorf("geo/parent = %q/%q", loc.GeoID, loc.ParentRef)
	}
	if len(loc.Hours) != 1 || loc.PhotoCount != 50000 {
		t.Errorf("hours/photos = %v/%d", loc.Hours, loc.PhotoCount)
	}
	if loc.Lat == 0 || loc.City != "Paris" {
		t.Errorf("lat/city = %v/%q", loc.Lat, loc.City)
	}
}

const apiReviews = `{"data":[
 {"id":847362910,"location_id":"188757","lang":"en","published_date":"2026-05-20","rating":"5","helpful_votes":"12","trip_type":"Couples","travel_date":"2026-04","title":"Magical","text":"Worth the climb.","user":{"username":"traveler99","user_location":{"name":"London"}},"owner_response":{"text":"Thank you!"}}
]}`

func TestAPIReviews(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(apiReviews))
	}))
	defer srv.Close()
	c := testClient(srv.URL)
	c.cfg.APIKey = "TESTKEY"
	rs, err := c.ReviewsAPI(context.Background(), "188757", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("got %d reviews", len(rs))
	}
	r := rs[0]
	if r.ID != "847362910" || r.Rating != 5 || r.Author != "traveler99" {
		t.Errorf("review = %+v", r)
	}
	if r.AuthorLoc != "London" || r.TripType != "Couples" || r.OwnerResp != "Thank you!" {
		t.Errorf("review fields = %+v", r)
	}
}

const apiPhotos = `{"data":[
 {"id":123456,"caption":"View from below","published_date":"2026-03-01","album":"Traveler photos","source":{"name":"Traveler"},"user":{"username":"shutterbug"},"images":{"thumbnail":{"url":"https://x/t.jpg","width":50,"height":50},"large":{"url":"https://x/l.jpg","width":550,"height":367},"original":{"url":"https://x/o.jpg","width":2000,"height":1333}}}
]}`

func TestAPIPhotos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(apiPhotos))
	}))
	defer srv.Close()
	c := testClient(srv.URL)
	c.cfg.APIKey = "TESTKEY"
	ps, err := c.PhotosAPI(context.Background(), "188757", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 {
		t.Fatalf("got %d photos", len(ps))
	}
	p := ps[0]
	if p.ID != "123456" || p.Location != "188757" || p.Source != "Traveler" {
		t.Errorf("photo = %+v", p)
	}
	if p.Original != "https://x/o.jpg" || p.Width != 2000 || p.Height != 1333 {
		t.Errorf("photo size = %q %d/%d", p.Original, p.Width, p.Height)
	}
}

func TestPlaneFor(t *testing.T) {
	cases := []struct {
		plane   string
		hasKey  bool
		webOK   bool
		apiOK   bool
		want    string
		wantErr bool
	}{
		{"auto", true, true, true, "api", false},
		{"auto", false, true, true, "web", false},
		{"auto", false, false, true, "", true}, // nearby, no key
		{"web", false, true, true, "web", false},
		{"web", true, true, true, "web", false}, // forced web overrides a key
		{"web", false, false, true, "", true},   // nearby forced to web
		{"api", true, true, true, "api", false},
		{"api", false, true, true, "", true}, // api forced, no key
	}
	for _, tc := range cases {
		c := NewClient(DefaultConfig())
		c.cfg.Plane = tc.plane
		if tc.hasKey {
			c.cfg.APIKey = "K"
		} else {
			c.cfg.APIKey = ""
		}
		got, err := c.planeFor(tc.webOK, tc.apiOK)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("planeFor(%v) plane=%s key=%v = (%q, %v)", tc, tc.plane, tc.hasKey, got, err)
		}
	}
}
