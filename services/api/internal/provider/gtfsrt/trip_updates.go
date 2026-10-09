package gtfsrt

import (
	"context"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"

	"singgah/services/api/internal/realtime"
)

// TripResolver maps a provider trip_id to the canonical Singgah trip UUID —
// the join key the departure board attaches delays to. nil leaves TripID
// empty and the store drops the update; an update we cannot join is noise.
type TripResolver func(ctx context.Context, providerTripID string) (string, bool)

// WithTripResolver attaches the trip resolver without changing NewClient's
// shape — feeds that only publish positions never need it.
func (c *Client) WithTripResolver(r TripResolver) *Client {
	c.resolveTrip = r
	return c
}

// FetchTripUpdates implements realtime.TripUpdateSource over the same feed
// URL — operators that publish a dedicated TripUpdates endpoint simply get
// a second Client pointed at it.
func (c *Client) FetchTripUpdates(ctx context.Context) ([]realtime.TripUpdate, error) {
	feed, err := c.fetchFeed(ctx)
	if err != nil {
		return nil, err
	}
	headerTS := feed.GetHeader().GetTimestamp()

	out := make([]realtime.TripUpdate, 0, len(feed.GetEntity()))
	for _, e := range feed.GetEntity() {
		tu := e.GetTripUpdate()
		if tu == nil {
			continue // vehicles/alerts are different feeds' jobs
		}
		ts := tu.GetTimestamp()
		if ts == 0 {
			ts = headerTS
		}
		u := realtime.TripUpdate{
			ObservedAt: time.Unix(int64(ts), 0).UTC(),
			Canceled:   tu.GetTrip().GetScheduleRelationship() == gtfs.TripDescriptor_CANCELED,
		}
		if c.resolveTrip != nil {
			if id, ok := c.resolveTrip(ctx, tu.GetTrip().GetTripId()); ok {
				u.TripID = id
			}
		}
		if tu.Delay != nil {
			d := tu.GetDelay()
			u.DelaySec = &d
		}
		for _, stu := range tu.GetStopTimeUpdate() {
			sd := realtime.StopDelay{
				StopKey: stu.GetStopId(),
				Skipped: stu.GetScheduleRelationship() == gtfs.TripUpdate_StopTimeUpdate_SKIPPED,
			}
			// Departure delay is the boarding truth; arrival is the
			// fallback when the feed reports only one event.
			switch {
			case stu.GetDeparture().GetDelay() != 0:
				sd.DelaySec = stu.GetDeparture().GetDelay()
			case stu.GetArrival().GetDelay() != 0:
				sd.DelaySec = stu.GetArrival().GetDelay()
			case stu.GetDeparture() != nil:
				sd.DelaySec = 0 // explicit on-time beats propagated guess
			case stu.GetArrival() != nil:
				sd.DelaySec = 0
			default:
				if !sd.Skipped {
					continue // NO_DATA with no event = nothing reported
				}
			}
			u.StopKeys = append(u.StopKeys, sd)
		}
		out = append(out, u)
	}
	return out, nil
}
