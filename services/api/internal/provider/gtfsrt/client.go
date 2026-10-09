// Package gtfsrt adapts a GTFS Realtime VehiclePositions feed into the
// canonical realtime.Vehicle shape. It is deliberately spec-conformant and
// operator-agnostic — no Jakarta-specific quirks are baked in, so whichever
// licensed feed lands first (TJ is the likely one) plugs in as config, not
// code. Wired into realtime.Poller only after the feed's terms are
// documented in docs/35_DATA_SOURCES.md.
package gtfsrt

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"
	"google.golang.org/protobuf/proto"

	"singgah/services/api/internal/realtime"
)

// maxFeedBytes bounds one fetch. Real-world VehiclePositions feeds are
// hundreds of KB; 32 MB already covers feeds orders of magnitude larger
// than Jakarta could produce.
const maxFeedBytes = 32 << 20

// RouteResolver maps a provider route_id to a canonical Singgah route UUID.
// The adapter parses feeds — it does not know the catalog. nil leaves
// RouteID empty, which downstream code already handles honestly.
type RouteResolver func(ctx context.Context, providerRouteID string) (string, bool)

type Client struct {
	url         string
	sourceID    string
	resolve     RouteResolver
	resolveTrip TripResolver
	http        *http.Client
}

func NewClient(url, sourceID string, resolve RouteResolver) *Client {
	return &Client{
		url:      url,
		sourceID: sourceID,
		resolve:  resolve,
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

// fetchFeed performs the one GET + protobuf decode shared by every feed
// type this client serves. A malformed payload is an error either way.
func (c *Client) fetchFeed(ctx context.Context) (*gtfs.FeedMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed %s: HTTP %d", c.sourceID, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes))
	if err != nil {
		return nil, fmt.Errorf("feed %s read: %w", c.sourceID, err)
	}
	var feed gtfs.FeedMessage
	if err := proto.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("feed %s parse: %w", c.sourceID, err)
	}
	return &feed, nil
}

// FetchVehicles implements realtime.Source: one GET, full parse, normalize.
// A malformed payload or a feed without any usable timestamp is a fetch
// error — entities were received but their freshness cannot be trusted, so
// the previous last-known state keeps serving instead of poisoning the cache.
func (c *Client) FetchVehicles(ctx context.Context) ([]realtime.Vehicle, error) {
	feed, err := c.fetchFeed(ctx)
	if err != nil {
		return nil, err
	}
	headerTS := feed.GetHeader().GetTimestamp()

	out := make([]realtime.Vehicle, 0, len(feed.GetEntity()))
	for _, e := range feed.GetEntity() {
		vp := e.GetVehicle()
		if vp == nil {
			continue // trip updates/alerts are different feeds' jobs
		}
		pos := vp.GetPosition()
		if pos == nil {
			continue // a vehicle entity without a position is data, not a defect
		}
		// Freshness truth: entity timestamp, else the feed header's.
		ts := vp.GetTimestamp()
		if ts == 0 {
			ts = headerTS
		}
		if ts == 0 {
			return nil, fmt.Errorf("feed %s: no entity or header timestamp — freshness unverifiable", c.sourceID)
		}
		v := realtime.Vehicle{
			ID:         vp.GetVehicle().GetId(),
			Source:     c.sourceID,
			TripID:     vp.GetTrip().GetTripId(),
			Lat:        float64(pos.GetLatitude()),
			Lon:        float64(pos.GetLongitude()),
			ObservedAt: time.Unix(int64(ts), 0).UTC(),
			ReceivedAt: time.Now().UTC(),
		}
		if v.ID == "" {
			v.ID = e.GetId()
		}
		if pos.Bearing != nil {
			b := float64(pos.GetBearing())
			v.Bearing = &b
		}
		if c.resolve != nil {
			if rid, ok := c.resolve(ctx, vp.GetTrip().GetRouteId()); ok {
				v.RouteID = rid
			}
		}
		out = append(out, v)
	}
	return out, nil
}
