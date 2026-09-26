// Package catalog serves the canonical transit network: station/route search
// and detail reads. It is the first domain surface under /api/v1.
package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/provider/commute"
)

// Store is the read seam the handlers depend on — satisfied by
// *generated.Queries, faked in unit tests.
type Store interface {
	SearchStops(ctx context.Context, arg generated.SearchStopsParams) ([]generated.SearchStopsRow, error)
	ListStops(ctx context.Context, limit int32) ([]generated.ListStopsRow, error)
	ListStopsInBBox(ctx context.Context, arg generated.ListStopsInBBoxParams) ([]generated.ListStopsInBBoxRow, error)
	GetStop(ctx context.Context, id pgtype.UUID) (generated.GetStopRow, error)
	ListRoutesServingStop(ctx context.Context, id pgtype.UUID) ([]generated.ListRoutesServingStopRow, error)
	ListTransfersFromStop(ctx context.Context, fromStopID pgtype.UUID) ([]generated.ListTransfersFromStopRow, error)
	ListRoutes(ctx context.Context, arg generated.ListRoutesParams) ([]generated.ListRoutesRow, error)
	GetRoute(ctx context.Context, id pgtype.UUID) (generated.GetRouteRow, error)
	ListStopsOnRoute(ctx context.Context, id pgtype.UUID) ([]generated.ListStopsOnRouteRow, error)
	ListProviders(ctx context.Context) ([]generated.ListProvidersRow, error)
	GetRouteByProviderEntityID(ctx context.Context, arg generated.GetRouteByProviderEntityIDParams) (pgtype.UUID, error)
	ListRouteLinesInBBox(ctx context.Context, arg generated.ListRouteLinesInBBoxParams) ([]generated.ListRouteLinesInBBoxRow, error)
}

// Timetabler fetches a station's scheduled departures inside an HH:MM window
// (Asia/Jakarta) — satisfied by *commute.Client, faked in tests.
type Timetabler interface {
	Timetable(ctx context.Context, operator, stationCode, from, to string) ([]commute.TimetableEntry, error)
}
