package tripadvisor

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions and
// the host wiring (mint, body, resolve), which need no network. The client's HTTP
// behaviour is covered in tripadvisor_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "tripadvisor" {
		t.Errorf("Scheme = %q, want tripadvisor", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want first %s", info.Hosts, Host)
	}
	if info.Identity.Binary != "ta" {
		t.Errorf("Identity.Binary = %q, want ta", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, kind, id string }{
		{"https://www.tripadvisor.com/Hotel_Review-g60763-d93450-Reviews-The_Plaza-New_York_City_New_York.html", "location", "93450"},
		{"https://www.tripadvisor.com/Restaurant_Review-g187147-d12947099-Reviews-Septime-Paris.html", "location", "12947099"},
		{"https://www.tripadvisor.com/Attraction_Review-g187147-d188757-Reviews-Eiffel_Tower-Paris.html", "location", "188757"},
		{"https://www.tripadvisor.com/ShowUserReviews-g60763-d93450-r123456789-The_Plaza.html", "location", "93450"},
		{"https://www.tripadvisor.com/Tourism-g187147-Paris_Ile_de_France-Vacations.html", "location", "187147"},
		{"https://www.tripadvisor.com/Hotels-g187147-Paris_Ile_de_France-Hotels.html", "location", "187147"},
		{"https://www.tripadvisor.com/robots.txt", "sitemaps", ""},
		{"https://www.tripadvisor.com/sitemap/att/en_US/sitemap_en_US_attractions_index.xml", "sitemap", "attractions"},
		{"https://www.tripadvisor.com/business/sitemap.xml", "sitemap", "business"},
		{"93450", "location", "93450"},
		{"g187147", "location", "187147"},
	}
	for _, tc := range cases {
		r := Classify(tc.in)
		if r.Kind != tc.kind || r.ID != tc.id {
			t.Errorf("Classify(%q) = (%q, %q), want (%q, %q)", tc.in, r.Kind, r.ID, tc.kind, tc.id)
		}
	}
}

func TestClassifyUnknown(t *testing.T) {
	for _, in := range []string{"", "not a url", "https://example.com/foo"} {
		if r := Classify(in); r.Kind != "unknown" {
			t.Errorf("Classify(%q).Kind = %q, want unknown", in, r.Kind)
		}
	}
}

func TestURLFor(t *testing.T) {
	cases := []struct{ kind, id, want string }{
		{"location", "93450", ContentURL + "/location/93450/details"},
		{"reviews", "93450", ContentURL + "/location/93450/reviews"},
		{"photos", "93450", ContentURL + "/location/93450/photos"},
		{"sitemaps", "", BaseURL + "/robots.txt"},
		{"sitemap", "attractions", BaseURL + "/sitemap/att/en_US/sitemap_en_US_attractions_index.xml"},
		{"sitemap", "business", BaseURL + "/business/sitemap.xml"},
	}
	for _, tc := range cases {
		if got := URLFor(tc.kind, tc.id); got != tc.want {
			t.Errorf("URLFor(%q, %q) = %q, want %q", tc.kind, tc.id, got, tc.want)
		}
	}
	if got := URLFor("sitemap", "no_such_kind"); got != "" {
		t.Errorf("URLFor for an unknown kind = %q, want empty", got)
	}
}

func TestDomainClassifyLocate(t *testing.T) {
	kind, id, err := Domain{}.Classify("93450")
	if err != nil || kind != "location" || id != "93450" {
		t.Fatalf("Domain.Classify = (%q, %q, %v)", kind, id, err)
	}
	got, err := Domain{}.Locate("location", "93450")
	if err != nil || got != ContentURL+"/location/93450/details" {
		t.Errorf("Domain.Locate = (%q, %v)", got, err)
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round trip: a record
// mints to its URI and a bare id resolves back to the same URI.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	loc := &Location{ID: "93450", Name: "The Plaza", URL: "https://www.tripadvisor.com/Hotel_Review-g60763-d93450.html"}
	u, err := h.Mint(loc)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "tripadvisor://location/93450"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	if !h.Searchable("tripadvisor") {
		t.Error("Searchable = false, want true (the domain registers a search op)")
	}

	got, err := h.ResolveOn("tripadvisor", "93450")
	if err != nil || got.String() != "tripadvisor://location/93450" {
		t.Errorf("ResolveOn = (%q, %v), want tripadvisor://location/93450", got.String(), err)
	}
}
