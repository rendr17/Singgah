// Package journey serves station-to-station itineraries. The MVP source is the
// Commute provider's fare/route endpoint (docs/23_ROUTING_MAP_GIS.md) —
// responses are normalized back onto canonical UUIDs and always labelled
// "scheduled": static provider data, never realtime.
package journey

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/provider/commute"
)

// Store is the read seam — satisfied by *generated.Queries, faked in tests.
type Store interface {
	GetStop(ctx context.Context, id pgtype.UUID) (generated.GetStopRow, error)
	ListStopIDsByProviderEntityIDs(ctx context.Context, arg generated.ListStopIDsByProviderEntityIDsParams) ([]generated.ListStopIDsByProviderEntityIDsRow, error)
	ListStopCoords(ctx context.Context, ids []pgtype.UUID) ([]generated.ListStopCoordsRow, error)
	GetRouteByProviderEntityID(ctx context.Context, arg generated.GetRouteByProviderEntityIDParams) (pgtype.UUID, error)
	SliceRouteShape(ctx context.Context, arg generated.SliceRouteShapeParams) (string, error)
	ListRouteAlternatives(ctx context.Context, arg generated.ListRouteAlternativesParams) ([]generated.ListRouteAlternativesRow, error)
	ListRouteStopSlice(ctx context.Context, arg generated.ListRouteStopSliceParams) ([]generated.ListRouteStopSliceRow, error)
}

// FarePlanner computes an upstream itinerary — satisfied by *commute.Client,
// faked in tests. at is the requested departure context (nil = leave now).
type FarePlanner interface {
	Fares(ctx context.Context, fromID, toID string, at *time.Time) (*commute.FarePlan, error)
}

// Timetabler fetches a station's scheduled departures inside an HH:MM window —
// satisfied by *commute.Client, faked in tests.
type Timetabler interface {
	Timetable(ctx context.Context, operator, stationCode, from, to string) ([]commute.TimetableEntry, error)
}
