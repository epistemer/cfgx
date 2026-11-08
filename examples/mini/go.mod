module github.com/epistemer/cfgx/examples/mini

go 1.26.0

require (
	github.com/epistemer/cfgx v0.0.0
	github.com/spf13/cobra v1.10.2
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	go.yaml.in/yaml/v4 v4.0.0 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
)

replace github.com/epistemer/cfgx => ../..

tool github.com/epistemer/cfgx/cmd/cfgx
