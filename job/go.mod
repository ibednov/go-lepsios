module github.com/ibednov/go-lepsios/job

go 1.25.0

require (
	github.com/ibednov/go-lepsios/identity v0.0.0
	github.com/ibednov/go-lepsios/log v0.0.0
	github.com/rs/zerolog v1.34.0
	github.com/spf13/cobra v1.9.1
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	golang.org/x/sys v0.47.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/ibednov/go-lepsios/identity => ../identity
	github.com/ibednov/go-lepsios/log => ../log
)
