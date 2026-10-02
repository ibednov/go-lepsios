package cbar

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ibednov/go-lepsios/log"
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

// FetchRatesOnDate GETs {base}/{DD.MM.YYYY}.xml
func (c *Client) FetchRatesOnDate(ctx context.Context, ondate time.Time) ([]xmlValute, error) {
	start := time.Now()
	reqURL := fmt.Sprintf("%s/%s.xml", c.baseURL, ondate.Format("02.01.2006"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		logSidecarFinished(reqURL, 0, 0, time.Since(start), err)
		return nil, err
	}
	req.Header.Set("Accept", "application/xml")
	req.Header.Set("User-Agent", "go-lepsios-exchange/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logSidecarFinished(reqURL, 0, 0, time.Since(start), err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("cbar: unexpected status %d", resp.StatusCode)
		logSidecarFinished(reqURL, resp.StatusCode, 0, time.Since(start), err)
		return nil, err
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		logSidecarFinished(reqURL, resp.StatusCode, 0, time.Since(start), err)
		return nil, err
	}

	items, err := parseXMLValutes(body)
	if err != nil {
		logSidecarFinished(reqURL, resp.StatusCode, 0, time.Since(start), err)
		return nil, err
	}
	logSidecarFinished(reqURL, resp.StatusCode, len(items), time.Since(start), nil)
	return items, nil
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
