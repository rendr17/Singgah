// Package osm fetches railway route geometry from OpenStreetMap route
// relations (type=route, route=train|subway|light_rail) via Overpass.
// Members arrive as way geometries that are stitched into continuous
// components — relation ways are unordered and may be reversed upstream.
// Used at ingest time only; Overpass is never a runtime dependency
// (docs/35_DATA_SOURCES.md).
package osm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
)

// DefaultBaseURL is a public Overpass endpoint — fine for occasional ingest
// runs; bulk use should move to a regional extract.
const DefaultBaseURL = "https://overpass-api.de/api/interpreter"

// DefaultAPIBaseURL is the main OSM editing API — the reliable fallback when
// public Overpass mirrors rate-limit or block the caller's network.
const DefaultAPIBaseURL = "https://api.openstreetmap.org/api/0.6"

// Point is one vertex, WGS84.
type Point struct {
	Lon float64
	Lat float64
}

// WKT renders a polyline as LINESTRING for st_geomfromtext.
func WKT(pts []Point) string {
	var b strings.Builder
	b.WriteString("LINESTRING(")
	for i, p := range pts {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%v %v", p.Lon, p.Lat)
	}
	b.WriteByte(')')
	return b.String()
}

// RouteRelations maps canonical route entity ids ("{operator}:{lineCode}",
// matching the commute provider's keys) to OSM route relations. Verified
// against OpenStreetMap 2026-09 — direction pairs and pattern variants are
// all stored; leg slicing picks the best snap per query.
var RouteRelations = map[string][]int64{
	// KAI Commuter (KCI) — route=train relations per direction/pattern.
	"KCI:A": {17675694, 17675695},                     // Lin Soekarno Hatta
	"KCI:B": {16877210, 16877211, 16877212, 16877213}, // Lin Bogor + Nambo
	"KCI:C": { // Lin Lingkar Cikarang: 2 full rackets + half-racket patterns
		2922163, 15094545,
		15097496, 15097497, 15097498, 15097499, 15097500, 15097501,
		15097502, 15097503, 15097504, 15097505, 15097506, 15097507,
	},
	"KCI:R":  {2922215, 15094546}, // Lin Rangkasbitung
	"KCI:T":  {2922235, 17193463}, // Lin Tangerang
	"KCI:TP": {2922237, 17193008}, // Lin Tanjung Priuk
	// MRT Jakarta.
	"MRTJ:M": {9677669, 9677670}, // North-South both directions
	// LRT Jabodebek.
	"LRTJBDB:CB": {16036440, 16036441},
	"LRTJBDB:BK": {16079478, 16079479},
	// LRT Jakarta (Lin Selatan).
	"LRTJ:S": {10693119, 10693160},
	// Kalayang Bandar Udara (Soekarno–Hatta people mover, route=light_rail).
	"APCGK:KLB": {15031928, 15031929}, // T3→T1 / T1→T3
	// TJ corridors absent from the TJ GTFS feed — OSM route=bus relations.
	"TJ:1N":  {17218709, 17237388}, // Metrotrans 1N Tanah Abang↔Blok M
	"TJ:10D": {17208295, 17208297}, // BRT 10D Kampung Rambutan↔Tanjung Priok
}

// ProviderRegistration is the providers-row payload for OSM — ODbL,
// attribution required. Geometry only; OSM is never a runtime source.
var ProviderRegistration = generated.UpsertProviderParams{
	Code:            "osm",
	Name:            "OpenStreetMap",
	SourceUrl:       pgtype.Text{String: "https://www.openstreetmap.org", Valid: true},
	TermsUrl:        pgtype.Text{String: "https://www.openstreetmap.org/copyright", Valid: true},
	LicenseName:     pgtype.Text{String: "ODbL-1.0", Valid: true},
	AttributionText: pgtype.Text{String: "Geometri jalur © kontributor OpenStreetMap (ODbL)", Valid: true},
	AllowedUse:      pgtype.Text{String: "route path geometry for map display", Valid: true},
	RefreshCadence:  pgtype.Text{String: "manual", Valid: true},
	Owner:           pgtype.Text{String: "OpenStreetMap contributors", Valid: true},
	KnownLimitations: pgtype.Text{
		String: "community-mapped route geometry; accuracy varies, relation coverage may drift upstream",
		Valid:  true,
	},
}

// RelationGeom is one OSM route relation resolved to drawable polylines.
// A relation whose ways don't all connect yields several Components —
// gaps stay as separate rows rather than inventing a straight bridge.
type RelationGeom struct {
	ID         int64
	Components [][]Point
}

// Client is a thin OSM caller — Overpass for bulk geom, main API as fallback.
type Client struct {
	baseURL string
	apiURL  string
	http    *http.Client
}

func NewClient(baseURL, apiURL string) *Client {
	return &Client{baseURL: baseURL, apiURL: apiURL, http: &http.Client{Timeout: 120 * time.Second}}
}

type overpassResponse struct {
	Elements []struct {
		Type    string `json:"type"`
		ID      int64  `json:"id"`
		Members []struct {
			Type     string `json:"type"`
			Geometry []struct {
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			} `json:"geometry"`
		} `json:"members"`
	} `json:"elements"`
}

// RelationGeoms fetches route relations by id in ONE Overpass call —
// `out geom` inlines each member way's vertex list.
func (c *Client) RelationGeoms(ctx context.Context, ids []int64) ([]RelationGeom, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	query := fmt.Sprintf("[out:json][timeout:100];rel(%s);out geom;", strings.Join(parts, ","))

	form := url.Values{"data": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("overpass: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("overpass: HTTP %d", resp.StatusCode)
	}
	var body overpassResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("overpass: decode: %w", err)
	}

	out := make([]RelationGeom, 0, len(body.Elements))
	for _, e := range body.Elements {
		var ways [][]Point
		for _, m := range e.Members {
			if m.Type != "way" || len(m.Geometry) < 2 {
				continue
			}
			pts := make([]Point, len(m.Geometry))
			for i, g := range m.Geometry {
				pts[i] = Point{Lon: g.Lon, Lat: g.Lat}
			}
			ways = append(ways, pts)
		}
		out = append(out, RelationGeom{ID: e.ID, Components: Stitch(ways)})
	}
	return out, nil
}

// RelationGeomsFull fetches relations one-by-one from the main OSM API
// (/relation/{id}/full) — slower than a single Overpass call but far more
// reliable: every member arrives with node coordinates inline. Used when
// Overpass mirrors refuse the client.
func (c *Client) RelationGeomsFull(ctx context.Context, ids []int64) ([]RelationGeom, error) {
	out := make([]RelationGeom, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			// Stay polite against the shared editing API.
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(300 * time.Millisecond):
			}
		}
		g, err := c.relationFull(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

type fullResponse struct {
	Elements []json.RawMessage `json:"elements"`
}

type osmNode struct {
	Type string  `json:"type"`
	ID   int64   `json:"id"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type osmWay struct {
	Type  string  `json:"type"`
	ID    int64   `json:"id"`
	Nodes []int64 `json:"nodes"`
}

type osmRel struct {
	Type    string `json:"type"`
	ID      int64  `json:"id"`
	Members []struct {
		Type string `json:"type"`
		Ref  int64  `json:"ref"`
		Role string `json:"role"`
	} `json:"members"`
}

func (c *Client) relationFull(ctx context.Context, id int64) (RelationGeom, error) {
	u := fmt.Sprintf("%s/relation/%d/full.json", c.apiURL, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return RelationGeom{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return RelationGeom{}, fmt.Errorf("osm api: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return RelationGeom{}, fmt.Errorf("osm api: rel %d: HTTP %d", id, resp.StatusCode)
	}
	var body fullResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return RelationGeom{}, fmt.Errorf("osm api: rel %d: decode: %w", id, err)
	}

	nodes := map[int64]Point{}
	ways := map[int64][]int64{}
	var memberOrder []int64
	for _, raw := range body.Elements {
		var head struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &head); err != nil {
			continue
		}
		switch head.Type {
		case "node":
			var n osmNode
			if json.Unmarshal(raw, &n) == nil {
				nodes[n.ID] = Point{Lon: n.Lon, Lat: n.Lat}
			}
		case "way":
			var w osmWay
			if json.Unmarshal(raw, &w) == nil {
				ways[w.ID] = w.Nodes
			}
		case "relation":
			var r osmRel
			if json.Unmarshal(raw, &r) == nil {
				for _, m := range r.Members {
					if m.Type == "way" {
						memberOrder = append(memberOrder, m.Ref)
					}
				}
			}
		}
	}

	var geoms [][]Point
	for _, wayID := range memberOrder {
		refs, ok := ways[wayID]
		if !ok {
			continue
		}
		pts := make([]Point, 0, len(refs))
		for _, ref := range refs {
			if p, ok := nodes[ref]; ok {
				pts = append(pts, p)
			}
		}
		if len(pts) >= 2 {
			geoms = append(geoms, pts)
		}
	}
	return RelationGeom{ID: id, Components: Stitch(geoms)}, nil
}

// stitchTolerance (~11m) covers survey gaps between adjacent way endpoints;
// shared nodes usually match exactly, the epsilon only absorbs rounding.
const stitchTolerance = 1e-4

func near(a, b Point) bool {
	dx := a.Lon - b.Lon
	dy := a.Lat - b.Lat
	return dx*dx+dy*dy < stitchTolerance*stitchTolerance
}

func reversed(w []Point) []Point {
	out := make([]Point, len(w))
	for i, p := range w {
		out[len(w)-1-i] = p
	}
	return out
}

// Stitch chains unordered (and possibly reversed) member ways into maximal
// continuous polylines. Ways that connect to no chain form their own
// component — a real gap must surface as a gap, not a fabricated segment.
func Stitch(ways [][]Point) [][]Point {
	pool := make([][]Point, 0, len(ways))
	for _, w := range ways {
		if len(w) >= 2 {
			pool = append(pool, w)
		}
	}
	var comps [][]Point
	for len(pool) > 0 {
		comp := pool[0]
		pool = pool[1:]
		for extended := true; extended; {
			extended = false
			for i, w := range pool {
				end, start := comp[len(comp)-1], comp[0]
				switch {
				case near(end, w[0]):
					comp = append(comp, w[1:]...)
				case near(end, w[len(w)-1]):
					comp = append(comp, reversed(w[:len(w)-1])...)
				case near(start, w[len(w)-1]):
					comp = append(w[:len(w)-1], comp...)
				case near(start, w[0]):
					comp = append(reversed(w[1:]), comp...)
				default:
					continue
				}
				pool = append(pool[:i], pool[i+1:]...)
				extended = true
				break
			}
		}
		comps = append(comps, comp)
	}
	return comps
}
