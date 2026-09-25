package catalystreference

import "list"

#SHA: string & =~"^[a-f0-9]{64}$"
#File: close({
	name: string & =~"^[A-Za-z0-9_.-]+$"
	sha256: #SHA
	bytes: int & >0 & <=16777216
})
#Reference: close({
	schema_version: 1
	id: "p10-hardened-rendered-reference-v1"
	source_manifest_sha256: "f8de11343773dbb8dbeb53bb713d684fee23fcd36604ddff921b07da57450ee2"
	original_archive_sha256: "94acb41796a8a97fdef44bf5b2d18f42919e43300cbca90077775f6b36dfa529"
	original_member: "catalyst-ui-kit/demo/typescript/src/app/(app)/settings/page.tsx"
	original_member_sha256: "1ca7f4e14738e917390815dc38d48047588eb324090d337f5c718b9d2ec6fa21"
	plan: #File
	default_html: #File
	error_html: #File
	countries: #File
	defaults: #File
	currencies: #File
	licenses: [#File]
	assets: [...close({url: string & =~"^/[^?#]+$", file: #File, mime: "text/css" | "font/woff" | "font/woff2" | "image/svg+xml"})] & list.MinItems(3) & list.MaxItems(128)
	frames: [...close({
		width: 390 | 1440
		theme: "light" | "dark"
		screenshot: #File
		geometry: close({width: number & >0, height: number & >0, inner: 390 | 1440, scroll: number & >0})
	})] & list.MinItems(4) & list.MaxItems(4)
	collector_sha256: #SHA
	chrome: "152.0.7977.84"
	playwright: "1.63.0"
	verification: "not-verified"
})
