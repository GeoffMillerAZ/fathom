package awscred

import "os"

// resolveRegion resolves the AWS region in priority order: explicit
// cfg.Region, then AWS_REGION, then AWS_DEFAULT_REGION, then sharedRegion
// (the selected shared-profile's "region" key, already extracted by
// loadSharedProfile — resolveRegion itself never touches the filesystem).
// Returns "" when none of these name a region; the caller (the bedrock
// adapter has no usable default region — GovCloud and commercial endpoints
// are distinct) decides whether an empty region is fatal.
func resolveRegion(cfg Config, sharedRegion string) string {
	return firstNonEmpty(cfg.Region, os.Getenv(envRegion), os.Getenv(envDefaultRegion), sharedRegion)
}
