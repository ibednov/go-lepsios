module github.com/ibednov/go-lepsios/parser

go 1.25.0

require (
	github.com/bogdanfinn/fhttp v0.6.9
	github.com/bogdanfinn/tls-client v1.16.0
	github.com/ibednov/go-lepsios/browser v0.1.1
	github.com/ibednov/go-lepsios/currency v0.3.0
	github.com/ibednov/go-lepsios/log v0.1.1
	github.com/stretchr/testify v1.11.1
	golang.org/x/text v0.32.0
)

require (
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/bdandy/go-errors v1.2.2 // indirect
	github.com/bdandy/go-socks4 v1.2.3 // indirect
	github.com/bogdanfinn/quic-go-utls v1.0.10-utls // indirect
	github.com/bogdanfinn/utls v1.7.8-barnius // indirect
	github.com/bogdanfinn/websocket v1.5.6-barnius // indirect
	github.com/cloudflare/circl v1.6.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/deckarep/golang-set/v2 v2.8.0 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mxschmitt/playwright-go v0.6201.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	github.com/tam7t/hpkp v0.0.0-20160821193359-2b70b4024ed5 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/ibednov/go-lepsios/browser => ../browser
	github.com/ibednov/go-lepsios/currency => ../currency
	github.com/ibednov/go-lepsios/log => ../log
)
