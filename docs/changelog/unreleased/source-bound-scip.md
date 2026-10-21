### Breaking

- `scip-dotnet` and sidecar-opted-in SCIP imports validate source receipts on import and refresh, refusing stale or incomplete inputs and invalidating only their typed namespace and graph memo; regenerate receipts and rebuild unbound older stores ([migration](../../upgrades/unreleased-source-bound-scip.md)).
