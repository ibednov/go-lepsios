package wildberries

import (
	"context"
	"sync/atomic"
	"testing"
)

func TestFindBasketHostParallelAndCached(t *testing.T) {
	const vol = 987654
	basketHostByVol.Delete(vol)
	want := "basket-31.wbbasket.ru"
	var calls atomic.Int32
	try := func(_ context.Context, host string) bool {
		calls.Add(1)
		return host == want
	}

	host, ok := findBasketHost(context.Background(), vol, try)
	if !ok || host != want {
		t.Fatalf("host=%q ok=%v", host, ok)
	}

	calls.Store(0)
	host, ok = findBasketHost(context.Background(), vol, try)
	if !ok || host != want || calls.Load() != 1 {
		t.Fatalf("host=%q ok=%v calls=%d, want one cached probe", host, ok, calls.Load())
	}
}

func TestFindBasketHostNotFound(t *testing.T) {
	const vol = 987655
	basketHostByVol.Delete(vol)
	if host, ok := findBasketHost(context.Background(), vol, func(context.Context, string) bool { return false }); ok {
		t.Fatalf("host=%q, want miss", host)
	}
}
