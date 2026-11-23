package wildberries

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const chromeTLSUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/152.0.0.0 Safari/537.36"

var errChromeTLSUnavailable = errors.New("chrome tls client unavailable")

type chromeTLSDoer func(ctx context.Context, reqURL, origin string) (status int, body []byte, err error)

var doChromeTLSGet chromeTLSDoer = chromeTLSGetJSON

var (
	chromeTLSOnce    sync.Once
	chromeTLSClient  tls_client.HttpClient
	chromeTLSInitErr error
)

func chromeTLS() (tls_client.HttpClient, error) {
	chromeTLSOnce.Do(func() {
		chromeTLSClient, chromeTLSInitErr = tls_client.NewHttpClient(
			tls_client.NewNoopLogger(),
			tls_client.WithTimeoutSeconds(8),
			tls_client.WithClientProfile(profiles.Chrome_152),
			tls_client.WithNotFollowRedirects(),
		)
	})
	if chromeTLSInitErr != nil {
		return nil, fmt.Errorf("%w: %v", errChromeTLSUnavailable, chromeTLSInitErr)
	}
	return chromeTLSClient, nil
}

func chromeTLSGetJSON(ctx context.Context, reqURL, origin string) (int, []byte, error) {
	client, err := chromeTLS()
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header = chromeTLSHeaders(origin)

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func chromeTLSHeaders(origin string) http.Header {
	if origin == "" {
		origin = "https://www.wildberries.ru"
	}
	return http.Header{
		"sec-ch-ua":          {`"Chromium";v="152", "Google Chrome";v="152", "Not A(Brand";v="24"`},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Linux"`},
		"user-agent":         {chromeTLSUserAgent},
		"accept":             {"application/json, text/plain, */*"},
		"origin":             {origin},
		"sec-fetch-site":     {"cross-site"},
		"sec-fetch-mode":     {"cors"},
		"sec-fetch-dest":     {"empty"},
		"referer":            {origin + "/"},
		"accept-encoding":    {"gzip, deflate, br, zstd"},
		"accept-language":    {"ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7"},
		http.HeaderOrderKey: {
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"sec-ch-ua-platform",
			"user-agent",
			"accept",
			"origin",
			"sec-fetch-site",
			"sec-fetch-mode",
			"sec-fetch-dest",
			"referer",
			"accept-encoding",
			"accept-language",
		},
	}
}

func wbCatalogOrigin(normalizedURL string) string {
	if wbMarketFromURL(normalizedURL).curr == "rub" {
		return "https://www.wildberries.ru"
	}
	return "https://www.wildberries.by"
}
