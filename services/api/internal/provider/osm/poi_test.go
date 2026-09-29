package osm

import (
	"strings"
	"testing"
)

func TestCategoryOf(t *testing.T) {
	cases := []struct {
		tags map[string]string
		want string
	}{
		{map[string]string{"amenity": "restaurant"}, "makan"},
		{map[string]string{"amenity": "cafe"}, "ngopi"},
		{map[string]string{"amenity": "cinema"}, "hiburan"},
		{map[string]string{"tourism": "museum"}, "budaya"},
		{map[string]string{"tourism": "zoo"}, "hiburan"},
		{map[string]string{"leisure": "park"}, "taman"},
		{map[string]string{"historic": "monument"}, "budaya"},
		{map[string]string{"shop": "mall"}, "belanja"},
		{map[string]string{"amenity": "marketplace"}, "belanja"},
		{map[string]string{"amenity": "place_of_worship"}, "budaya"},
		// amenity beats tourism when both present (documented precedence).
		{map[string]string{"amenity": "cafe", "tourism": "museum"}, "ngopi"},
		{map[string]string{"amenity": "bench"}, ""},
		{map[string]string{}, ""},
	}
	for _, c := range cases {
		if got := CategoryOf(c.tags); got != c.want {
			t.Errorf("CategoryOf(%v) = %q want %q", c.tags, got, c.want)
		}
	}
}

func TestPOIQuery(t *testing.T) {
	q := POIQuery(JabodetabekBBox, true)
	// Every mapped tag value must appear in the query — the mapping table is
	// the single source of truth for the fetch set.
	for key, vals := range categoryByTag {
		if !strings.Contains(q, `"`+key+`"~`) {
			t.Fatalf("query missing tag key %q: %s", key, q)
		}
		for v := range vals {
			if !strings.Contains(q, v) {
				t.Fatalf("query missing value %q under %q: %s", v, key, q)
			}
		}
	}
	if !strings.Contains(q, `["name"]`) {
		t.Fatal("query must require a name tag")
	}
	if !strings.Contains(q, "out meta center;") {
		t.Fatal("meta query must request meta + center")
	}
	if got := POIQuery(JabodetabekBBox, false); !strings.Contains(got, "out center;") || strings.Contains(got, "meta") {
		t.Fatalf("non-meta fallback query malformed: %s", got)
	}
}

func TestCategoryValuesStayInCheckConstraint(t *testing.T) {
	// Schema CHECK allows only this set — a mapping to anything else fails
	// at insert time; catch drift here instead.
	allowed := map[string]bool{
		"makan": true, "ngopi": true, "hiburan": true,
		"taman": true, "budaya": true, "belanja": true, "other": true,
	}
	for key, vals := range categoryByTag {
		for v, cat := range vals {
			if !allowed[cat] {
				t.Fatalf("categoryByTag[%s][%s] = %q not in schema CHECK set", key, v, cat)
			}
		}
	}
}
