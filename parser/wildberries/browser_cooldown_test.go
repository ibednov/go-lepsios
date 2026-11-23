package wildberries

import (
	"context"
	"errors"
	"testing"
)

type countingCapturer struct {
	calls int
	err   error
}

func (c *countingCapturer) CaptureJSON(context.Context, string, string) ([]byte, error) {
	c.calls++
	if c.err != nil {
		return nil, c.err
	}
	return []byte(`{"products":[{"name":"X","sizes":[{"price":{"product":14288}}]}]}`), nil
}

func TestCaptureDetailRetriesBrowserEveryCall(t *testing.T) {
	capt := &countingCapturer{err: errors.New("context deadline exceeded")}
	s := NewStrategy(capt)
	url := "https://www.wildberries.ru/catalog/1/detail.aspx"

	if src := s.captureDetail(context.Background(), url); src.err == nil || capt.calls != 1 {
		t.Fatalf("calls=%d err=%v", capt.calls, src.err)
	}
	capt.err = nil
	src := s.captureDetail(context.Background(), url)
	if src.err != nil || capt.calls != 2 {
		t.Fatalf("calls=%d err=%v, want a second capture with no cooldown", capt.calls, src.err)
	}
	_, price, _, err := extractProductInfo(src.data, url)
	if err != nil || price != 142.88 {
		t.Fatalf("price=%v err=%v", price, err)
	}
}
