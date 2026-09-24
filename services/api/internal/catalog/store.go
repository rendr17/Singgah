// Package catalog serves the canonical transit network: station/route search
// and detail reads. It is the first domain surface under /api/v1.
package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	generated "singgah/services/api/db/generated"
)

// Store is the read seam the handlers depend on — satisfied by
// *generated.Queries, faked in unit tests.
type Store interface {
	SearchStops(ctx context.Context, arg generated.SearchStopsParams) ([]generated.SearchStopsRow, error)
	GetStop(ctx context.Context, id pgtype.UUID) (generated.GetStopRow, error)
	ListRoutesServingStop(ctx context.Context, id pgtype.UUID) ([]generated.ListRoutesServingStopRow, error)
	ListTransfersFromStop(ctx context.Context, fromStopID pgtype.UUID) ([]generated.ListTransfersFromStopRow, error)
	ListRoutes(ctx context.Context, arg generated.ListRoutesParams) ([]generated.ListRoutesRow, error)
	GetRoute(ctx context.Context, id pgtype.UUID) (generated.GetRouteRow, error)
	ListStopsOnRoute(ctx context.Context, id pgtype.UUID) ([]generated.ListStopsOnRouteRow, error)
}
