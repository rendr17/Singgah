// POI import for City Explorer (ADR-011): bounded Overpass queries at ingest
// time only. The Overpass query is BUILT FROM categoryByTag so the fetch set
// and the mapping table can never drift — values we don't map are never
// fetched.
package osm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Canonical place categories (docs/14 launch set + 'other'). Keys are the OSM
// tag key; inner map is tag value → category. Checked in tag-key order —
// amenity wins over tourism, etc. — so e.g. a museum cafe categorizes as
// 'ngopi', not 'budaya'; that ordering trade-off is documented here.
var categoryByTag = map[string]map[string]string{
	"amenity": {
		"restaurant": "makan", "fast_food": "makan", "food_court": "makan",
		"cafe":   "ngopi",
		"cinema": "hiburan", "theatre": "hiburan", "arts_centre": "hiburan",
		"marketplace":      "belanja",
		"place_of_worship": "budaya", "library": "budaya", "community_centre": "budaya",
	},
	"tourism": {
		"museum": "budaya", "gallery": "budaya",
		"zoo": "hiburan", "aquarium": "hiburan", "theme_park": "hiburan", "attraction": "hiburan",
	},
	"leisure": {
		"park": "taman", "garden": "taman", "nature_reserve": "taman",
	},
	"historic": {
		"monument": "budaya", "memorial": "budaya", "castle": "budaya",
		"ruins": "budaya", "archaeological_site": "budaya",
	},
	"shop": {
		"mall": "belanja", "department_store": "belanja",
		"supermarket": "belanja", "convenience": "belanja",
	},
}

// tagQueryOrder fixes iteration order so the generated Overpass query (and
// CategoryOf precedence) is deterministic.
var tagQueryOrder = []string{"amenity", "tourism", "leisure", "historic", "shop"}

// CategoryOf maps OSM tags to one canonical category, "" when nothing maps.
func CategoryOf(tags map[string]string) string {
	for _, key := range tagQueryOrder {
		if cat, ok := categoryByTag[key][tags[key]]; ok {
			return cat
		}
	}
	return ""
}

// BBox is an Overpass area bound in (south,west,north,east) order.
type BBox struct{ South, West, North, East float64 }

// JabodetabekBBox covers metro Jakarta + Bodetabek conservatively.
var JabodetabekBBox = BBox{South: -7.05, West: 106.35, North: -5.90, East: 107.25}

// POIQuery builds the bounded Overpass QL for all mapped tag values with a
// name — unnamed POIs are useless for the explorer UI. `out meta center`
// yields tags + element timestamp + a center point for ways/relations; if a
// mirror rejects it the caller retries with `out center` (no timestamps).
func POIQuery(box BBox, meta bool) string {
	var q strings.Builder
	q.WriteString("[out:json][timeout:120];(")
	bb := fmt.Sprintf("(%v,%v,%v,%v)", box.South, box.West, box.North, box.East)
	for _, key := range tagQueryOrder {
		vals := make([]string, 0, len(categoryByTag[key]))
		for v := range categoryByTag[key] {
			vals = append(vals, v)
		}
		sort.Strings(vals)
		re := "^(" + strings.Join(vals, "|") + ")$"
		fmt.Fprintf(&q, `nwr[%q~%q]["name"]%s;`, key, re, bb)
	}
	q.WriteString(");")
	if meta {
		q.WriteString("out meta center;")
	} else {
		q.WriteString("out center;")
	}
	return q.String()
}

// POI is one named OSM element resolved to a point + canonical category.
type POI struct {
	EntityID  string // "node:123" | "way:456" | "relation:789"
	Name      string
	Category  string
	Lat       float64
	Lon       float64
	Tags      map[string]string
	UpdatedAt time.Time // zero when the mirror doesn't return meta
}

type overpassPOIResponse struct {
	Elements []struct {
		Type   string  `json:"type"`
		ID     int64   `json:"id"`
		Lat    float64 `json:"lat"`
		Lon    float64 `json:"lon"`
		Center *struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"center"`
		Tags      map[string]string `json:"tags"`
		Timestamp string            `json:"timestamp"`
	} `json:"elements"`
}

// POIs fetches all named, canonically-categorized elements inside bbox.
// Retries without meta when the mirror rejects `out meta center`.
func (c *Client) POIs(ctx context.Context, b BBox) ([]POI, error) {
	pois, err := c.pois(ctx, b, true)
	if err != nil {
		pois, err = c.pois(ctx, b, false)
	}
	return pois, err
}

// postOverpass is the same form POST used by RelationGeoms, generalized to a
// decode target so POI and shape calls share the wire path.
func (c *Client) postOverpass(ctx context.Context, query string, out any) error {
	form := url.Values{"data": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Public mirrors 406 the generic Go UA — a descriptive UA is required.
	req.Header.Set("User-Agent", "Singgah/ingest-places (OpenStreetMap data import)")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("overpass: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("overpass: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("overpass: decode: %w", err)
	}
	return nil
}

func (c *Client) pois(ctx context.Context, b BBox, meta bool) ([]POI, error) {
	var resp overpassPOIResponse
	if err := c.postOverpass(ctx, POIQuery(b, meta), &resp); err != nil {
		return nil, err
	}
	out := make([]POI, 0, len(resp.Elements))
	for _, e := range resp.Elements {
		lat, lon := e.Lat, e.Lon
		if e.Type != "node" {
			if e.Center == nil {
				continue // way/relation without center — skip rather than fake coords
			}
			lat, lon = e.Center.Lat, e.Center.Lon
		}
		cat := CategoryOf(e.Tags)
		if cat == "" || e.Tags["name"] == "" {
			continue
		}
		p := POI{
			EntityID: fmt.Sprintf("%s:%d", e.Type, e.ID),
			Name:     e.Tags["name"],
			Category: cat,
			Lat:      lat,
			Lon:      lon,
			Tags:     e.Tags,
		}
		if t, err := time.Parse(time.RFC3339, e.Timestamp); err == nil {
			p.UpdatedAt = t
		}
		out = append(out, p)
	}
	return out, nil
}
