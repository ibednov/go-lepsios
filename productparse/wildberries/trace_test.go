package wildberries

import (
	"context"
	"strings"
	"testing"
)

func TestTruncateWBBodyPrefersLDJSON(t *testing.T) {
	body := strings.Repeat("x", 300) + `<script type="application/ld+json">[{"name":"Пиалы"}]</script>`
	got := truncateWBBody(body)
	if !strings.HasPrefix(got, "application/ld+json") {
		t.Fatalf("body=%q", got)
	}
	if len(got) > wbTraceBodyLimit {
		t.Fatalf("len=%d", len(got))
	}
}

func TestRecordWBNoopsWithoutTrace(t *testing.T) {
	recordWBHTTP(context.Background(), "napi", "GET", "https://example", 200, []byte("ok"), nil)
}
