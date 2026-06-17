package tripadvisor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// hotelLD is a captured-shape JSON-LD island for a hotel page, with the loose field
// types schema.org allows (a number quoted, an author as an object, two images in an
// array). The place island and the page's BreadcrumbList share one @graph wrapper,
// the shape TripAdvisor serves, so ldNodes has to unwrap it to reach both.
const hotelLD = `<html><head>
<script type="application/ld+json">
{"@context":"https://schema.org","@graph":[
{"@type":"Hotel","name":"The Plaza",
 "description":"A landmark hotel on Fifth Avenue.","priceRange":"$$$$",
 "telephone":"+1 212-759-3000",
 "address":{"@type":"PostalAddress","streetAddress":"768 5th Ave","addressLocality":"New York City","addressRegion":"NY","postalCode":"10019","addressCountry":{"@type":"Country","name":"United States"}},
 "geo":{"@type":"GeoCoordinates","latitude":"40.7644","longitude":"-73.9742"},
 "aggregateRating":{"@type":"AggregateRating","ratingValue":"4.5","reviewCount":"5436"},
 "image":["https://media.tacdn.com/plaza-1.jpg","https://media.tacdn.com/plaza-2.jpg"],
 "review":[{"@type":"Review","author":{"@type":"Person","name":"Jane T"},"name":"Wonderful stay","reviewBody":"Beautiful rooms and great service.","datePublished":"2026-05-01","inLanguage":"en","reviewRating":{"@type":"Rating","ratingValue":5}}]},
{"@type":"BreadcrumbList","itemListElement":[
 {"@type":"ListItem","position":1,"name":"United States","item":{"@id":"https://www.tripadvisor.com/Tourism-g191-United_States.html","name":"United States"}},
 {"@type":"ListItem","position":2,"name":"New York","item":{"@id":"https://www.tripadvisor.com/Tourism-g28953-New_York.html","name":"New York"}},
 {"@type":"ListItem","position":3,"name":"New York City","item":{"@id":"https://www.tripadvisor.com/Tourism-g60763-New_York_City.html","name":"New York City"}},
 {"@type":"ListItem","position":4,"name":"The Plaza","item":{"@id":"https://www.tripadvisor.com/Hotel_Review-g60763-d93450-Reviews-The_Plaza.html","name":"The Plaza"}}]}
]}
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
	if loc.Image != "https://media.tacdn.com/plaza-1.jpg" {
		t.Errorf("image = %q", loc.Image)
	}
	// The BreadcrumbList in the @graph supplies the geo lineage, nearest first, with
	// the place's own crumb dropped.
	wantAnc := []string{"60763", "28953", "191"}
	if len(loc.Ancestors) != len(wantAnc) {
		t.Fatalf("ancestors = %+v, want ids %v", loc.Ancestors, wantAnc)
	}
	for i, id := range wantAnc {
		if loc.Ancestors[i].ID != id {
			t.Errorf("ancestor[%d] id = %q, want %q", i, loc.Ancestors[i].ID, id)
		}
	}
	if strings.Join(loc.AncestorRefs, ",") != strings.Join(wantAnc, ",") {
		t.Errorf("ancestor_refs = %v, want %v", loc.AncestorRefs, wantAnc)
	}
	if loc.Ancestors[0].Name != "New York City" {
		t.Errorf("nearest ancestor name = %q", loc.Ancestors[0].Name)
	}
}

func TestImagesFromLD(t *testing.T) {
	imgs := imagesFromLD([]byte(hotelLD))
	want := []string{"https://media.tacdn.com/plaza-1.jpg", "https://media.tacdn.com/plaza-2.jpg"}
	if strings.Join(imgs, ",") != strings.Join(want, ",") {
		t.Errorf("images = %v, want %v", imgs, want)
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
 "location_id":"188757","name":"Eiffel Tower","local_name":"Tour Eiffel","description":"Paris landmark.",
 "web_url":"https://www.tripadvisor.com/Attraction_Review-g187147-d188757.html",
 "see_all_photos":"https://www.tripadvisor.com/LocationPhotoDirectLink-g187147-d188757.html",
 "write_review":"https://www.tripadvisor.com/UserReview-g187147-d188757.html",
 "address_obj":{"street1":"Champ de Mars","city":"Paris","country":"France","postalcode":"75007","address_string":"Champ de Mars, 5 Avenue Anatole France, 75007 Paris France"},
 "local_address":"Champ de Mars, 75007 Paris",
 "ancestors":[{"level":"City","name":"Paris","location_id":"187147"},{"level":"Country","name":"France","location_id":"187070"}],
 "neighborhood_info":[{"name":"7th Arrondissement","location_id":"15621101"},{"name":"Gros Caillou","location_id":"15621102"}],
 "latitude":"48.8584","longitude":"2.2945","timezone":"Europe/Paris",
 "rating":"4.5","num_reviews":"140126","rating_image_url":"https://www.tripadvisor.com/img/cdsi/img2/ratings/4.5.svg",
 "review_rating_count":{"1":"800","2":"1200","3":"9000","4":"40000","5":"89126"},
 "subratings":{"0":{"name":"value","localized_name":"Value","value":"4.0"},"1":{"name":"atmosphere","localized_name":"Atmosphere","value":"5.0"}},
 "ranking_data":{"ranking_string":"#1 of 3,236 things to do in Paris","ranking":"1","ranking_out_of":"3236"},
 "category":{"name":"attraction","localized_name":"Attraction"},
 "subcategory":[{"name":"sights","localized_name":"Sights & Landmarks"}],
 "groups":[{"name":"Attractions","categories":[{"name":"Observation Decks & Towers","localized_name":"Observation Decks & Towers"},{"name":"Points of Interest & Landmarks","localized_name":"Points of Interest & Landmarks"}]}],
 "features":["Wheelchair Accessible","Gift Shop"],
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
	if loc.LocalName != "Tour Eiffel" || loc.LocalAddress != "Champ de Mars, 75007 Paris" {
		t.Errorf("local name/address = %q/%q", loc.LocalName, loc.LocalAddress)
	}
	if loc.SeeAllPhotos == "" || loc.WriteReview == "" {
		t.Errorf("photo/review URLs = %q/%q", loc.SeeAllPhotos, loc.WriteReview)
	}
	// The two ancestors with ids climb the geo tree; ancestor_refs mirrors them.
	wantAnc := []string{"187147", "187070"}
	if strings.Join(loc.AncestorRefs, ",") != strings.Join(wantAnc, ",") {
		t.Errorf("ancestor_refs = %v, want %v", loc.AncestorRefs, wantAnc)
	}
	if len(loc.Ancestors) != 2 || loc.Ancestors[0].Level != "City" {
		t.Errorf("ancestors = %+v", loc.Ancestors)
	}
	// The neighborhoods descend the tree; neighborhood_refs mirrors them.
	wantNbr := []string{"15621101", "15621102"}
	if strings.Join(loc.NeighborhoodRefs, ",") != strings.Join(wantNbr, ",") {
		t.Errorf("neighborhood_refs = %v, want %v", loc.NeighborhoodRefs, wantNbr)
	}
	if loc.RatingCounts["5"] != 89126 || loc.RatingCounts["1"] != 800 {
		t.Errorf("rating_counts = %v", loc.RatingCounts)
	}
	if len(loc.Subratings) != 2 || loc.Subratings[0].Name != "Value" || loc.Subratings[0].Value != 4.0 {
		t.Errorf("subratings = %+v", loc.Subratings)
	}
	if len(loc.Groups) != 2 || loc.Groups[0] != "Observation Decks & Towers" {
		t.Errorf("groups = %v", loc.Groups)
	}
	if len(loc.Features) != 2 || loc.Features[0] != "Wheelchair Accessible" {
		t.Errorf("features = %v", loc.Features)
	}
}

const apiReviews = `{"data":[
 {"id":847362910,"location_id":"188757","lang":"en","published_date":"2026-05-20","rating":"5","helpful_votes":"12","trip_type":"Couples","travel_date":"2026-04","title":"Magical","text":"Worth the climb.","is_machine_translated":true,"subratings":{"0":{"name":"value","localized_name":"Value","value":"4.0"},"1":{"name":"service","localized_name":"Service","value":"5.0"}},"user":{"username":"traveler99","user_location":{"name":"London"}},"owner_response":{"text":"Thank you!"}}
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
	if !r.Translated {
		t.Error("review machine_translated not carried")
	}
	if len(r.Subratings) != 2 || r.Subratings[0].Name != "Value" || r.Subratings[1].Value != 5.0 {
		t.Errorf("review subratings = %+v", r.Subratings)
	}
}

const apiPhotos = `{"data":[
 {"id":123456,"caption":"View from below","published_date":"2026-03-01","album":"Traveler photos","is_blessed":true,"source":{"name":"Traveler"},"user":{"username":"shutterbug"},"images":{"thumbnail":{"url":"https://x/t.jpg","width":50,"height":50},"large":{"url":"https://x/l.jpg","width":550,"height":367},"original":{"url":"https://x/o.jpg","width":2000,"height":1333}}}
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
	if !p.Blessed {
		t.Error("photo is_blessed not carried")
	}
}

// TestWebPhotos confirms the web photos fallback emits every island image, not just
// a single hero, each tagged back to its location.
func TestWebPhotos(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(hotelLD))
	}))
	defer srv.Close()
	c := testClient(srv.URL)
	ps, err := c.WebPhotos(context.Background(), srv.URL+"/Hotel_Review-g60763-d93450-Reviews-The_Plaza.html", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 {
		t.Fatalf("got %d photos, want 2", len(ps))
	}
	if ps[0].Location != "93450" || ps[0].Original != "https://media.tacdn.com/plaza-1.jpg" {
		t.Errorf("photo[0] = %+v", ps[0])
	}
	if ps[1].Original != "https://media.tacdn.com/plaza-2.jpg" {
		t.Errorf("photo[1] = %+v", ps[1])
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
