// Tailwind CSS standalone binary pin (D2: no Node, ever).
//
// The single source of truth for which Tailwind the kit runs. The runner
// (tailwind.go) embeds and validates this file at startup; a checksum
// mismatch on a downloaded binary is a hard failure, not a warning.
//
// To bump: update version + every sha256 from the release's sha256sums.txt
// (https://github.com/tailwindlabs/tailwindcss/releases), then run
// `task check` — the conformance test builds real CSS through the new pin.
//
// Checksums below are from the official v4.3.0 sha256sums.txt asset,
// fetched 2026-06-10.
package tailwind

manifest: {
	version: "v4.3.0"

	// keyed by Go's runtime.GOOS-runtime.GOARCH
	binaries: {
		"darwin-arm64": {
			asset:  "tailwindcss-macos-arm64"
			sha256: "56b4bbc62dbdc4614a78930d9c6986423a2ec63e4e640144a59a5d95c914322e"
		}
		"darwin-amd64": {
			asset:  "tailwindcss-macos-x64"
			sha256: "2ba252f770817091e6d0d12a84e0dd531bcc29aad1bfd9d976a3aff1a071b67a"
		}
		"linux-arm64": {
			asset:  "tailwindcss-linux-arm64"
			sha256: "8f48dcb72be3b351c10563c5329b4638ba8516820dc3b3a1609625a166e87cbd"
		}
		"linux-amd64": {
			asset:  "tailwindcss-linux-x64"
			sha256: "73f0e5459054e5cfaa8ab6f3b940f3fbe0f13cc7fd83bc24e7c655033c203400"
		}
		"windows-amd64": {
			asset:  "tailwindcss-windows-x64.exe"
			sha256: "e4303d06a337899d426583508f55c143ce89a585f90ed080f482d02fd83093c1"
		}
	}
}

// Schema — the manifest above must conform.
manifest: #Manifest

#Manifest: {
	version: =~"^v[0-9]+\\.[0-9]+\\.[0-9]+$"
	binaries: [=~"^[a-z0-9]+-[a-z0-9]+$"]: {
		asset:  string & !=""
		sha256: =~"^[a-f0-9]{64}$"
	}
}
