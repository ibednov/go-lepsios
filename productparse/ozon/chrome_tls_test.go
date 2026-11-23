package ozon

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFetchProductTLSTakesComposerPrice(t *testing.T) {
	prev := doChromeTLSGet
	t.Cleanup(func() { doChromeTLSGet = prev })
	doChromeTLSGet = func(_ context.Context, reqURL, _ string) (int, []byte, error) {
		if strings.Contains(reqURL, "composer-api") {
			return 200, []byte(sampleComposerJSON), nil
		}
		return 200, []byte("<html></html>"), nil
	}

	product, err := fetchProductTLS(
		context.Background(),
		"https://ozon.by/product/shlyuz-zigbee-3-0-tuya-smart-life-besprovodnoy-hab-wifi-dlya-umnogo-doma-1102861861/",
		composerAPIURL("https://ozon.by/", "/product/shlyuz-zigbee-3-0-tuya-smart-life-besprovodnoy-hab-wifi-dlya-umnogo-doma-1102861861/"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if product.Title == "" || product.Price != 53022 || product.Currency != "RUB" {
		t.Fatalf("product=%+v", product)
	}
}

func TestFetchProductTLSKeepsChallengeAsAccessDenied(t *testing.T) {
	prev := doChromeTLSGet
	t.Cleanup(func() { doChromeTLSGet = prev })
	doChromeTLSGet = func(context.Context, string, string) (int, []byte, error) {
		return 403, []byte(`{"incidentId":"x","challengeURL":"https://ozon.by/challenge.html"}`), nil
	}

	_, err := fetchProductTLS(
		context.Background(),
		"https://ozon.by/product/shlyuz-zigbee-3-0-tuya-smart-life-besprovodnoy-hab-wifi-dlya-umnogo-doma-1102861861/",
		"https://ozon.by/api/composer-api.bx/page/json/v2?url=%2Fproduct%2Fx%2F",
	)
	if err == nil || !errors.Is(err, errAccessDenied) {
		t.Fatalf("err=%v", err)
	}
}

func TestChromeTLSClientConstructs(t *testing.T) {
	client, err := chromeTLS()
	if err != nil {
		t.Fatalf("chrome TLS client: %v", err)
	}
	if client == nil {
		t.Fatal("chrome TLS client is nil")
	}
}
