module github.com/geoffmilleraz/fathom/calibration/telemetry-census

go 1.26.2

require (
	github.com/geoffmilleraz/fathom v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require github.com/kr/pretty v0.3.1 // indirect

// The census reads PromQL with fathom's own selector scanner, from this checkout.
replace github.com/geoffmilleraz/fathom => ../..
