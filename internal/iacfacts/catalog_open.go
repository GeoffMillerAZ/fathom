package iacfacts

import "strings"

// OpenIaCCatalogManifest reads a catalog's manifest and checks it whole: format, kind, source, line, and a version
// list that is valid, strictly ascending and inside the line. It reads no version index and no blob, so it is cheap,
// and it is what the public door (pkg/lang/iac.OpenCatalog) validates a catalog by.
func OpenIaCCatalogManifest(path string) (IaCCatalogManifest, error) {
	m, err := ReadIaCCatalogManifest(path)
	if err != nil {
		return m, err
	}
	if err := checkManifestVersions(m); err != nil {
		return m, err
	}
	return m, nil
}

// SameProviderSource reports whether a catalog built for catalogSource answers for a provider address the caller
// wrote: namespace and name equal, and the registry hosts equal or both one of the two default registries
// (design iac-facts.md section 5). An address with no host is on the default registry. Addresses compare lower case.
func SameProviderSource(catalogSource, source string) bool {
	if strings.TrimSpace(source) == "" || strings.TrimSpace(catalogSource) == "" {
		return false
	}
	return sameProvider(strings.ToLower(catalogSource), normalizeProviderSource(source, ""))
}
