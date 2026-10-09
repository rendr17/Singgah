package gtfsrt

import (
	"context"
	"strings"
	"time"

	"github.com/MobilityData/gtfs-realtime-bindings/golang/gtfs"

	"singgah/services/api/internal/realtime"
)

// FetchAlerts implements realtime.AlertSource over the same feed URL.
// Operators that publish a dedicated alerts endpoint simply get a second
// Client pointed at it — same normalization either way.
func (c *Client) FetchAlerts(ctx context.Context) ([]realtime.Alert, error) {
	feed, err := c.fetchFeed(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]realtime.Alert, 0, len(feed.GetEntity()))
	for _, e := range feed.GetEntity() {
		a := e.GetAlert()
		if a == nil {
			continue // vehicles/trip updates belong to their own fetches
		}
		al := realtime.Alert{
			ID:          e.GetId(),
			Source:      c.sourceID,
			Cause:       strings.ToLower(a.GetCause().String()),
			Effect:      strings.ToLower(a.GetEffect().String()),
			Header:      pickTranslation(a.GetHeaderText()),
			Description: pickTranslation(a.GetDescriptionText()),
			URL:         pickTranslation(a.GetUrl()),
		}
		for _, tr := range a.GetActivePeriod() {
			var p realtime.Period
			if tr.Start != nil && tr.GetStart() != 0 {
				t := time.Unix(int64(tr.GetStart()), 0).UTC()
				p.Start = &t
			}
			if tr.End != nil && tr.GetEnd() != 0 {
				t := time.Unix(int64(tr.GetEnd()), 0).UTC()
				p.End = &t
			}
			al.Periods = append(al.Periods, p)
		}
		al.Scoped = len(a.GetInformedEntity()) > 0
		seen := map[string]bool{}
		for _, sel := range a.GetInformedEntity() {
			rid := sel.GetRouteId()
			if rid == "" || seen[rid] {
				continue
			}
			seen[rid] = true
			if c.resolve != nil {
				if canon, ok := c.resolve(ctx, rid); ok {
					al.RouteIDs = append(al.RouteIDs, canon)
				}
			}
		}
		out = append(out, al)
	}
	return out, nil
}

// pickTranslation prefers Bahasa Indonesia, then English, then whatever the
// feed offers first — the feed's vocabulary, not ours.
func pickTranslation(ts *gtfs.TranslatedString) string {
	if ts == nil {
		return ""
	}
	var first, en string
	for _, tr := range ts.GetTranslation() {
		if first == "" {
			first = tr.GetText()
		}
		lang := strings.ToLower(tr.GetLanguage())
		if lang == "id" || strings.HasPrefix(lang, "id-") {
			return tr.GetText()
		}
		if lang == "en" || strings.HasPrefix(lang, "en-") {
			en = tr.GetText()
		}
	}
	if en != "" {
		return en
	}
	return first
}
