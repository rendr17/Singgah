// Package ingest runs a provider catalog refresh: fetch -> validate ->
// normalize -> upsert inside one transaction. A failed run commits nothing.
package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	generated "singgah/services/api/db/generated"
	"singgah/services/api/internal/provider/commute"
)

// Source is the seam between the network client and the pipeline — the real
// implementation is commute.Client; tests feed fixtures through it.
type Source interface {
	Operators(ctx context.Context) ([]commute.Operator, error)
	Stations(ctx context.Context) ([]commute.Station, error)
	Transfers(ctx context.Context, operatorCode, stationCode string) ([]commute.Transfer, error)
}

// Report summarizes one ingest run — processed counts plus every rejected
// record. Ingest never fails silently on bad provider data.
type Report struct {
	Operators  int
	Stops      int
	Routes     int
	Transfers  int
	Rejections []commute.Rejection
	FetchedAt  time.Time
}

// Commute runs the Commute Data Platform catalog refresh.
func Commute(ctx context.Context, pool *pgxpool.Pool, src Source) (Report, error) {
	fetchedAt := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	report := Report{FetchedAt: fetchedAt.Time}

	// Provider registration and the attempt stamp commit OUTSIDE the ingest
	// transaction: a failed run must still leave a trace on the health
	// surface, which only works if these two writes are not rolled back.
	q0 := generated.New(pool)
	provider, err := q0.UpsertProvider(ctx, commute.ProviderRegistration)
	if err != nil {
		return report, fmt.Errorf("ingest: provider registration: %w", err)
	}
	if err := q0.TouchProviderLastAttempt(ctx, commute.ProviderCode); err != nil {
		return report, fmt.Errorf("ingest: last_attempt_at: %w", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return report, fmt.Errorf("ingest: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := q0.WithTx(tx)

	operators, err := src.Operators(ctx)
	if err != nil {
		return report, fmt.Errorf("ingest: fetch operators: %w", err)
	}
	stations, err := src.Stations(ctx)
	if err != nil {
		return report, fmt.Errorf("ingest: fetch stations: %w", err)
	}

	// stopIDs maps provider entity ids to canonical UUIDs so transfers can
	// resolve their targets within the same ingest run.
	stopIDs := make(map[string]pgtype.UUID, len(stations))
	stationCodes := make(map[string]string, len(stations)) // id -> operator code, for transfer fetches

	for _, op := range operators {
		params := commute.NormalizeOperator(op, fetchedAt)
		params.ProviderID = provider.ID
		agency, err := q.UpsertAgency(ctx, params)
		if err != nil {
			return report, fmt.Errorf("ingest: agency %s: %w", op.Code, err)
		}
		report.Operators++

		for _, line := range op.Lines {
			rp := commute.NormalizeLine(op, line, agency.ID, fetchedAt)
			rp.ProviderID = provider.ID
			if _, err := q.UpsertRoute(ctx, rp); err != nil {
				return report, fmt.Errorf("ingest: route %s:%s: %w", op.Code, line.LineCode, err)
			}
			report.Routes++
		}
	}

	for _, st := range stations {
		params, rej := commute.NormalizeStation(st, fetchedAt)
		if rej != nil {
			report.Rejections = append(report.Rejections, *rej)
			continue
		}
		params.ProviderID = provider.ID
		stop, err := q.UpsertStop(ctx, params)
		if err != nil {
			return report, fmt.Errorf("ingest: stop %s: %w", st.ID, err)
		}
		stopIDs[st.ID] = stop.ID
		stationCodes[st.ID] = st.Operator
		report.Stops++
	}

	// Transfers are fetched per station — the provider has no bulk endpoint.
	// A station whose transfer fetch fails fails the run: silently partial
	// transfer graphs are worse than no run at all.
	for _, st := range stations {
		opCode, ok := stationCodes[st.ID]
		if !ok {
			continue // station was rejected during normalize
		}
		if st.Code == "" {
			// The transfers endpoint is keyed by station code — a missing
			// code is a data issue, so reject this station's edges instead
			// of letting a malformed upstream path fail the whole run.
			report.Rejections = append(report.Rejections, commute.Rejection{
				EntityID: st.ID,
				Reason:   "missing station code — transfers not fetchable",
			})
			continue
		}
		transfers, err := src.Transfers(ctx, opCode, st.Code)
		if err != nil {
			return report, fmt.Errorf("ingest: transfers for %s: %w", st.ID, err)
		}
		for _, tr := range transfers {
			params, rej := commute.NormalizeTransfer(tr, stopIDs)
			if rej != nil {
				report.Rejections = append(report.Rejections, *rej)
				continue
			}
			params.FromStopID = stopIDs[st.ID]
			params.FetchedAt = fetchedAt
			if _, err := q.UpsertTransfer(ctx, params); err != nil {
				return report, fmt.Errorf("ingest: transfer %s: %w", tr.ID, err)
			}
			report.Transfers++
		}
	}

	if err := q.TouchProviderLastSuccess(ctx, commute.ProviderCode); err != nil {
		return report, fmt.Errorf("ingest: last_success_at: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return report, fmt.Errorf("ingest: commit: %w", err)
	}
	return report, nil
}
