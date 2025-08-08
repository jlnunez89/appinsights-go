module adaptive-sampling-example

go 1.23.1

replace github.com/microsoft/ApplicationInsights-Go => ../..

require github.com/microsoft/ApplicationInsights-Go v0.0.0-00010101000000-000000000000

require (
	code.cloudfoundry.org/clock v1.38.0 // indirect
	github.com/gofrs/uuid/v5 v5.3.2 // indirect
)
