package browser

import "time"

const (
	defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	defaultTimeout   = 45 * time.Second
	defaultWarmup    = 20 * time.Second
)

// Options launches headless Chromium for parser JSON capture.
type Options struct {
	Headless       bool
	Timeout        time.Duration
	Warmup         time.Duration
	UserAgent      string
	ExecutablePath string
	MaxConcurrent  int
}

func (o Options) withDefaults() Options {
	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}
	if o.Warmup < 0 {
		o.Warmup = 0
	} else if o.Warmup == 0 {
		o.Warmup = defaultWarmup
	}
	if o.UserAgent == "" {
		o.UserAgent = defaultUserAgent
	}
	if o.MaxConcurrent <= 0 {
		o.MaxConcurrent = 1
	}
	return o
}
