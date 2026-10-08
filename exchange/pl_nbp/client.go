package pl_nbp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/log"
)

const (
	lookbackA = 7  // table A is business-daily
	lookbackB = 14 // table B is roughly weekly
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 12 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		httpClient: httpClient,
	}
}

// FetchRatesOnDate loads NBP tables A+B for ondate (looks back on weekends/holidays).
// Returns merged mid rates and the effective date from table A (or B if A missing).
func (c *Client) FetchRatesOnDate(ctx context.Context, ondate time.Time) ([]rateDTO, time.Time, error) {
	day := time.Date(ondate.Year(), ondate.Month(), ondate.Day(), 0, 0, 0, 0, time.UTC)

	aItems, aDate, errA := c.fetchTableWithLookback(ctx, "A", day, lookbackA)
	bItems, bDate, errB := c.fetchTableWithLookback(ctx, "B", day, lookbackB)
	if errA != nil && errB != nil {
		return nil, time.Time{}, fmt.Errorf("nbp: table A: %v; table B: %v", errA, errB)
	}

	merged := make([]rateDTO, 0, len(aItems)+len(bItems))
	seen := make(map[string]struct{}, len(aItems)+len(bItems))
	for _, item := range aItems {
		code := strings.ToUpper(strings.TrimSpace(item.Code))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		merged = append(merged, item)
	}
	for _, item := range bItems {
		code := strings.ToUpper(strings.TrimSpace(item.Code))
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		merged = append(merged, item)
	}
	if len(merged) == 0 {
		return nil, time.Time{}, fmt.Errorf("nbp: empty merged rates for %s", day.Format("2006-01-02"))
	}

	effective := aDate
	if effective.IsZero() {
		effective = bDate
	}
	return merged, effective, nil
}

func (c *Client) fetchTableWithLookback(ctx context.Context, table string, day time.Time, lookback int) ([]rateDTO, time.Time, error) {
	var lastErr error
	for i := 0; i <= lookback; i++ {
		d := day.AddDate(0, 0, -i)
		items, effective, err := c.fetchTable(ctx, table, d)
		if err == nil {
			return items, effective, nil
		}
		lastErr = err
		if !isNotFound(err) {
			return nil, time.Time{}, err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("nbp: no data for table %s near %s", table, day.Format("2006-01-02"))
	}
	return nil, time.Time{}, lastErr
}

func (c *Client) fetchTable(ctx context.Context, table string, day time.Time) ([]rateDTO, time.Time, error) {
	start := time.Now()
	reqURL := fmt.Sprintf("%s/exchangerates/tables/%s/%s/?format=json", c.baseURL, table, day.Format("2006-01-02"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		logSidecarFinished(reqURL, 0, 0, time.Since(start), err)
		return nil, time.Time{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "go-lepsios-exchange/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logSidecarFinished(reqURL, 0, 0, time.Since(start), err)
		return nil, time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		err := fmt.Errorf("nbp: not found %s %s", table, day.Format("2006-01-02"))
		logSidecarFinished(reqURL, resp.StatusCode, 0, time.Since(start), err)
		return nil, time.Time{}, err
	}
	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("nbp: unexpected status %d", resp.StatusCode)
		logSidecarFinished(reqURL, resp.StatusCode, 0, time.Since(start), err)
		return nil, time.Time{}, err
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		logSidecarFinished(reqURL, resp.StatusCode, 0, time.Since(start), err)
		return nil, time.Time{}, err
	}

	var tables []tableDTO
	if err := json.Unmarshal(body, &tables); err != nil {
		logSidecarFinished(reqURL, resp.StatusCode, 0, time.Since(start), err)
		return nil, time.Time{}, fmt.Errorf("nbp: json unmarshal: %w", err)
	}
	if len(tables) == 0 {
		err := fmt.Errorf("nbp: empty table %s", table)
		logSidecarFinished(reqURL, resp.StatusCode, 0, time.Since(start), err)
		return nil, time.Time{}, err
	}

	effective := day
	if tables[0].EffectiveDate != "" {
		if parsed, err := time.Parse("2006-01-02", tables[0].EffectiveDate); err == nil {
			effective = parsed
		}
	}
	logSidecarFinished(reqURL, resp.StatusCode, len(tables[0].Rates), time.Since(start), nil)
	return tables[0].Rates, effective, nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}

func logSidecarFinished(reqURL string, httpStatus, ratesCount int, duration time.Duration, err error) {
	fields := []interface{}{
		"component", "currency",
		"stage", "sidecar",
		"provider", ProviderID,
		"operation", "fetch_rates",
		"request_url", reqURL,
		"http_status", httpStatus,
		"rates_count", ratesCount,
		"duration_ms", duration.Milliseconds(),
		"success", err == nil,
	}
	if err != nil {
		fields = append(fields, "error", err.Error())
		log.Error("exchange.sidecar.finished", fields...)
		return
	}
	log.Info("exchange.sidecar.finished", fields...)
}
