package ozon

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

type chromeTLSDoer func(ctx context.Context, reqURL, referer string) (status int, body []byte, err error)

var doChromeTLSGet chromeTLSDoer = chromeTLSGet

var (
	chromeTLSOnce    sync.Once
	chromeTLSClient  tls_client.HttpClient
	chromeTLSInitErr error
)

func chromeTLS() (tls_client.HttpClient, error) {
	chromeTLSOnce.Do(func() {
		chromeTLSClient, chromeTLSInitErr = tls_client.NewHttpClient(
			tls_client.NewNoopLogger(),
			tls_client.WithTimeoutSeconds(15),
			tls_client.WithClientProfile(profiles.Chrome_152),
			tls_client.WithCookieJar(tls_client.NewCookieJar()),
		)
	})
	if chromeTLSInitErr != nil {
		return nil, fmt.Errorf("%w: %v", errChromeTLSUnavailable, chromeTLSInitErr)
	}
	return chromeTLSClient, nil
}

func chromeTLSGet(ctx context.Context, reqURL, referer string) (int, []byte, error) {
	client, err := chromeTLS()
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header = chromeTLSHeaders(referer)

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 6<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func chromeTLSHeaders(referer string) http.Header {
	if referer == "" {
		referer = "https://ozon.by/"
	}
	return http.Header{
		"sec-ch-ua":          {`"Chromium";v="152", "Google Chrome";v="152", "Not A(Brand";v="24"`},
		"sec-ch-ua-mobile":   {"?0"},
		"sec-ch-ua-platform": {`"Linux"`},
		"user-agent":         {chromeTLSUserAgent},
		"accept":             {"application/json, text/html, */*;q=0.8"},
		"sec-fetch-site":     {"same-origin"},
		"sec-fetch-mode":     {"navigate"},
		"sec-fetch-dest":     {"document"},
		"referer":            {referer},
		"accept-encoding":    {"gzip, deflate, br, zstd"},
		"accept-language":    {"ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7"},
		http.HeaderOrderKey: {
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"sec-ch-ua-platform",
			"user-agent",
			"accept",
			"sec-fetch-site",
			"sec-fetch-mode",
			"sec-fetch-dest",
			"referer",
			"accept-encoding",
			"accept-language",
		},
	}
}
