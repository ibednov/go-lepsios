module github.com/ibednov/go-lepsios/exchange

go 1.25.0

require (
	github.com/ibednov/go-lepsios/currency v0.0.0-20260823135235-df098bfd0b1e
	github.com/ibednov/go-lepsios/log v0.1.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/zerolog v1.34.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/ibednov/go-lepsios/currency => ../currency
	github.com/ibednov/go-lepsios/log => ../log
)
