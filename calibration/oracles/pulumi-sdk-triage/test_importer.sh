#!/usr/bin/env bash
# Run serially at actual nice >=10 and load <=9, as required by the lane.
set -euo pipefail
mkdir -p workspace/tmp/pulumi-sdk-helper
cp calibration/oracles/pulumi-sdk-triage/importer.go.txt workspace/tmp/pulumi-sdk-helper/main.go
cp calibration/oracles/pulumi-sdk-triage/importer_test.go.txt workspace/tmp/pulumi-sdk-helper/main_test.go
GOWORK=off GOPROXY=off GOSUMDB=off python3 calibration/oracles/pulumi-sdk-triage/scheduled.py go test -race ./workspace/tmp/pulumi-sdk-helper
