package commute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// DefaultBaseURL is the public Commute Data Platform API.
const DefaultBaseURL = "https://api.commute.shiorilabs.id"

// Client fetches provider payloads. It is deliberately thin — normalization
// lives in normalize.go and is exercised against fixtures, not the network.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("commute: build request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("commute: %s: %w", path, err)
	}
	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var env envelope[json.RawMessage]
		var e struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if err := dec.Decode(&env); err == nil && json.Unmarshal(env.Error, &e) == nil && e.Message != "" {
			switch e.Code {
			case "UNKNOWN_STATION":
				return ErrStationUnknown
			case "NO_TOPOLOGY":
				return ErrNoTopology
			}
			return fmt.Errorf("commute: %s: %s", path, e.Message)
		}
		return fmt.Errorf("commute: %s: HTTP %d", path, resp.StatusCode)
	}
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("commute: %s: decode: %w", path, err)
	}
	return nil
}

func (c *Client) Operators(ctx context.Context) ([]Operator, error) {
	var env envelope[[]Operator]
	if err := c.get(ctx, "/operators", &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

func (c *Client) Stations(ctx context.Context) ([]Station, error) {
	var env envelope[[]Station]
	if err := c.get(ctx, "/stations", &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// Transfers lists outgoing transfers from one station — the provider only
// exposes this per station, so ingest calls it once per known station.
func (c *Client) Transfers(ctx context.Context, operatorCode, stationCode string) ([]Transfer, error) {
	var env envelope[[]Transfer]
	path := fmt.Sprintf("/stations/%s/%s/transfers", operatorCode, stationCode)
	if err := c.get(ctx, path, &env); err != nil {
		return nil, err
	}
	return env.Data, nil
}

// LineDetail returns a line's declared segments — the ordered stop sequence
// ingest persists into route_stops. One call per line (~111 per full ingest).
func (c *Client) LineDetail(ctx context.Context, operatorCode, lineCode string) (*LineDetail, error) {
	var env envelope[LineDetail]
	path := fmt.Sprintf("/lines/%s/%s", url.PathEscape(operatorCode), url.PathEscape(lineCode))
	if err := c.get(ctx, path, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}

// ErrNoTopology reports the provider's 404 NO_TOPOLOGY — it publishes no
// declared stop sequence for this line (common for BRT corridors). Ingest
// treats this as honest absence, not failure: the route keeps an empty
// sequence rather than failing the whole run.
var ErrNoTopology = errors.New("commute: no topology for line")

// ErrStationUnknown reports the provider's 404 UNKNOWN_STATION — the pair is
// syntactically valid but unresolvable upstream (e.g. a stop we rejected at
// ingest). The journey handler maps it to an empty plan, not a 500.
var ErrStationUnknown = errors.New("commute: station unknown upstream")

// Fares computes the provider's station-to-station itinerary — the MVP journey
// source per docs/23_ROUTING_MAP_GIS.md.
func (c *Client) Fares(ctx context.Context, fromID, toID string) (*FarePlan, error) {
	var env envelope[FarePlan]
	path := fmt.Sprintf("/fares/%s/%s", url.PathEscape(fromID), url.PathEscape(toID))
	if err := c.get(ctx, path, &env); err != nil {
		return nil, err
	}
	return &env.Data, nil
}
