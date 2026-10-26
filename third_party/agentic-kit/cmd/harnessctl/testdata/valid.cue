package harnessctl

registration: #Registration & {
 version: "harnessctl.registration/1"
 epoch: 1
 issuedAt: "2026-10-24T00:00:00Z"
 expiresAt: "2026-10-25T00:00:00Z"
 hostId: "host"
 keys: [{id: "issuer", publicKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", revoked: false}]
 controls: [{id: "review", event: "before-tool", class: "advisory", minSamplePPM: 0, maxSamplePPM: 1000000, defaultSamplePPM: 500000, maxWorkMillis: 2000, onFault: "warn", allowMaintenance: false, tunableBy: ["installation"]}]
 projects: [{id: "project", root: "/protected/project", teamIds: [], principalId: "person", controlIds: ["review"], layers: [{id: "installation", kind: "installation", targetId: "host", path: "/protected/layer.json", keyId: "issuer", revision: 1, payloadSHA256: "0000000000000000000000000000000000000000000000000000000000000000", maxAgeSeconds: 3600, priority: 0, controlIds: ["review"]}]}]
}
layer: #Layer & {
 version: "harnessctl.layer/1"
 id: "installation"
 kind: "installation"
 targetId: "host"
 revision: 1
 issuedAt: "2026-10-24T00:00:00Z"
 expiresAt: "2026-10-25T00:00:00Z"
 controls: [{id: "review", minSamplePPM: 100000, maxSamplePPM: 900000, maxWorkMillis: 1000, onFault: "hold", allowMaintenance: false, tunableBy: ["installation"]}]
 preferences: [{controlId: "review", samplePPM: 700000}]
}
failure: #Report & {version: "harnessctl.report/1", status: "hold", category: "source", code: "registration_invalid", expectedLayers: 0, loadedLayers: 0, authenticatedLayers: 0, noActionAuthority: true, coverage: "selected-project-only", installation: "unobserved"}
check: #Report & {version: "harnessctl.report/1", status: "authenticated", noActionAuthority: true, coverage: "selected-project-only", installation: "unobserved", registrationEpoch: 1, registrationSHA256: "0000000000000000000000000000000000000000000000000000000000000000", snapshotSHA256: "0000000000000000000000000000000000000000000000000000000000000000", issuedAt: "2026-10-24T00:00:00Z", expiresAt: "2026-10-25T00:00:00Z", layerCount: 1, controlCount: 1}
