// Package journey serves station-to-station itineraries computed by the
// in-house schedule planner (internal/planner) over the ingested
// services/trips/stop_times/frequencies/transfers tables. The provider's
// fare endpoint is consulted only as a fare reference — optional context
// that never gates the plan. Responses carry canonical UUIDs and honest
// status: "scheduled" for published times, "estimated" for frequency
// templates and reconstructed stop_times — never realtime.
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
	SliceRouteShape(ctx context.Context, arg generated.SliceRouteShapeParams) (string, error)
	ListRouteAlternatives(ctx context.Context, arg generated.ListRouteAlternativesParams) ([]generated.ListRouteAlternativesRow, error)
	ListRouteStopSlice(ctx context.Context, arg generated.ListRouteStopSliceParams) ([]generated.ListRouteStopSliceRow, error)
	ListRouteColors(ctx context.Context, ids []pgtype.UUID) ([]generated.ListRouteColorsRow, error)
}

// FarePlanner provides the provider's corridor fare estimate — satisfied by
// *commute.Client, faked in tests. at is the requested departure context
// (nil = leave now); the result is reference context only.
type FarePlanner interface {
	Fares(ctx context.Context, fromID, toID string, at *time.Time) (*commute.FarePlan, error)
}
