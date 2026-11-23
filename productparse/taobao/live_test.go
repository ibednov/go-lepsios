//go:build live

package taobao

import (
	"context"
	"testing"
	"time"
)

func TestLiveParseTaobaoShareShoes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	res, err := NewStrategy("").ParseCard(ctx, "https://e.tb.cn/h.8QIkCHR7m0pUjke?tk=tcNHT2B1zad")
	if err != nil {
		t.Fatal(err)
	}
	if res.Title == "" {
		t.Fatal("empty title")
	}
	if res.Price == nil || *res.Price != 19.9 {
		t.Fatalf("price=%v", res.Price)
	}
	if res.Currency != "CNY" {
		t.Fatalf("currency=%q", res.Currency)
	}
}
