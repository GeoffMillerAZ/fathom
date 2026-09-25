package designreceipts_test

// navbarOverlaySpec selects reviewed P09 historical bytes. Current transformer
// files are snapshots, not a reconstructed execution closure. The baseline zoom
// receipt remains a failed interaction observation; later scoped repair/runtime
// results do not promote any full gate.
func navbarOverlaySpec() overlaySpec {
	return overlaySpec{
		Name:              "navbar",
		Subject:           "reference.navbar-adapter",
		Member:            "html/navigation/navbars/04-simple-with-menu-button-on-left.html",
		SourceSHA:         "79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7",
		ArtifactSHA:       "67f015c1efa6c41b2acb38d56ac00ecc24b4c7d406729655d8330d425a5a6137",
		InteractionReason: "The unchanged base actual-200% run records 38 failures: 24 conservative focus-screen failures, four long-label geometry failures, and ten runtime-loss access failures. Later explicit repair/runtime and delivered-browser passes are scoped observations, not full interaction, lifecycle, accessibility, performance, consumer acceptance, or target certification.",
		Assets: []overlayAsset{
			{ID: "navbar-source", Group: "source", Root: "navbar", Path: "original.html", SHA: "79cdbf1cd163a13cb77cc96c4cf8046a088ad8f18757658408372d437400b9d7"},
			{ID: "navbar-source-css", Group: "baseline", Root: "navbar", Path: "browser-Tn5991/source.css", SHA: "708b958629b406aa74d951cd80d10096e1f5ee6ea7af6d5c855a6e26dfe3952c"},

			{ID: "navbar-transformer", Group: "transformer", Root: "kit", Path: "internal/designadapt/navbar.go", SHA: "6cca622e15633fa42e9d1032dbe3af78c579a20da0e62caab99c7f54f1211a7e"},
			{ID: "navbar-schema", Group: "transformer", Root: "kit", Path: "internal/designadapt/navbar.cue", SHA: "5144a7de62241ded5eede9890cf3c2032bff42f79090fd93ec03500cbff1b737"},
			{ID: "navbar-repair-transformer", Group: "transformer", Root: "kit", Path: "internal/designadapt/navbar_repair.go", SHA: "52c9f2e8b9a983fcd33ed9f51576b93fef48cab1e6019ee269a114263254b63f"},
			{ID: "navbar-repair-schema", Group: "transformer", Root: "kit", Path: "internal/designadapt/navbar_repair.cue", SHA: "84cd4860fbff7f5c77e1f861d86954d893ed3b66056bdcca03b4f4cec88cad09"},
			{ID: "navbar-runtime-transformer", Group: "transformer", Root: "kit", Path: "internal/designadapt/navbar_runtime.go", SHA: "f8b5f9d370f0551814ec5722f02399ee6654d49e41183ce65a0ee61500d9b04c"},
			{ID: "navbar-runtime-schema", Group: "transformer", Root: "kit", Path: "internal/designadapt/navbar_runtime.cue", SHA: "3cf1fee2b4244f8c6aca9215d5e4d93a3d9a690efe58122afcb0002e55ec069d"},
			{ID: "navbar-delivery-transformer", Group: "transformer", Root: "kit", Path: "internal/designadapt/navbar_repair_delivery.go", SHA: "a419fa10de81a3bf74411f5fe6a70a54c4a0ca4a081d498e48a72758357a01b8"},
			{ID: "navbar-delivery-schema", Group: "transformer", Root: "kit", Path: "internal/designadapt/navbar_repair_delivery.cue", SHA: "93ae6b790c62b1e9f23c135fd689565639ec5d82b44af6dee8223c94375a5f73"},

			{ID: "navbar-delivery-usage", Group: "output", Root: "delivery", Path: "USAGE.md", SHA: "7d7a8b6cf16018640383304e5ad15dcb0042daf78334388ef14b7568e3979244"},
			{ID: "navbar-delivery-artifact", Group: "output", Root: "delivery", Path: "artifact.json", SHA: "67f015c1efa6c41b2acb38d56ac00ecc24b4c7d406729655d8330d425a5a6137"},
			{ID: "navbar-delivery-defaults", Group: "output", Root: "delivery", Path: "defaults.json", SHA: "944638f8e9128c7843c36597431f477e02cbcda6edb58960d109294586f4217c"},
			{ID: "navbar-delivery-runtime", Group: "output", Root: "delivery", Path: "elements-navbar.js", SHA: "7c7184f9ea5ba3e4842f8624f500cb80d81c3c8621570715cfbd69ecdf1d7179"},
			{ID: "navbar-delivery-model", Group: "output", Root: "delivery", Path: "navbar.go", SHA: "72f2199cbda35ea6b6707e2de07dc773fef4734e41e8b734707acfc723f0b8da"},
			{ID: "navbar-delivery-template", Group: "output", Root: "delivery", Path: "navbar.templ", SHA: "e790c9d506a90f7aa3c0287c257468123be0f38ab0e535b26e60123180216f94"},
			{ID: "navbar-delivery-repair-css", Group: "patch", Root: "delivery", Path: "navbar_repair.css", SHA: "c00d7a477ba6872cbc82ff73bffde2d2f71ad481fc888cd590f5bb9894f30bf0"},
			{ID: "navbar-delivery-routes-model", Group: "patch", Root: "delivery", Path: "navbar_routes.go", SHA: "b7be22be2fd55f5fd7d589174e61a8d09894f54d980240991e802c6dbe8fb562"},
			{ID: "navbar-delivery-routes-template", Group: "patch", Root: "delivery", Path: "navbar_routes.templ", SHA: "7c9dbfd0696781fa9a09d54c5edf788464bca259cda82053324c84348fb6bf86"},

			{ID: "navbar-elements-runtime", Group: "runtime", Root: "runtime", Path: "elements.js", SHA: "120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013"},
			{ID: "navbar-inter-font", Group: "runtime", Root: "runtime", Path: "inter.woff2", SHA: "693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3"},

			{ID: "navbar-browser-harness", Group: "fixture", Root: "kit", Path: "docs/assessments/fixtures/navbar-adapter/browser.mjs", SHA: "7388a1e05b53a4eb907081ba449af0aa043167ac88718256d50d60d977348c3e"},
			{ID: "navbar-zoom-harness", Group: "fixture", Root: "kit", Path: "docs/assessments/fixtures/navbar-adapter/zoom.mjs", SHA: "8b840c6c978d6a2ebc1820c76d9decd4eb063948b1ed12b39a30f25d28b1c2f3"},
			{ID: "navbar-runtime-zoom-harness", Group: "fixture", Root: "kit", Path: "docs/assessments/fixtures/navbar-adapter/runtime-zoom.mjs", SHA: "e3d9a204bfcca37a8b59ffac7da91d66e1b0dcf2247075ebf378f9e2ef2c4b85"},
			{ID: "navbar-runtime-zoom-runner", Group: "fixture", Root: "runtime-proof", Path: "zoom-runner-C4nmNn/run.mjs", SHA: "c9bb84b7641ea751d337d5e8e43709a9761f89b028197c684694047d36e4069a"},
			{ID: "navbar-delivery-browser-harness", Group: "fixture", Root: "delivery-run", Path: "harness.mjs", SHA: "752369888f7718b547046baeb33ec994c0c30a1e35b40e9d736dfd8ece6a697c"},

			{ID: "navbar-compile-receipt", Group: "toolchain", Root: "navbar", Path: "compile-receipt.json", SHA: "d2d19352a2ef101db2a8351c1015afaedbb4ee4b0daf438669b5d45585072062"},
			{ID: "navbar-runtime-receipt", Group: "toolchain", Root: "runtime-proof", Path: "runtime-receipt.json", SHA: "e8e4e06247e1ea3a95fc63dc931c3ca4dcf19a957e962500f5743db8262cfa5b"},
			{ID: "navbar-delivery-receipt", Group: "toolchain", Root: "delivery-parent", Path: "delivery-receipt.json", SHA: "1192e4761ebd57eb6f1baadaf894da9b4bec73fd3bbde28161c75e2615c2972b"},
		},
		Runs: []overlayRun{
			{ID: "navbar-base", Gate: "visual", Root: "navbar", Path: "browser-Tn5991/receipt.json", SHA: "2dfad48ffd46e6187ab4809b3a7dd65675a1af48fdbf1702c1e01ca1102dbe48", Images: 20},
			{ID: "navbar-baseline-zoom", Gate: "interaction", Root: "navbar", Path: "zoom-t4U6Hs/failure.json", SHA: "d537109542046601845d0e53f45dbf6dea92efc74abe0138490cd1da731e412f", Images: 34},
			{ID: "navbar-runtime-zoom", Gate: "lifecycle", Root: "runtime-proof", Path: "runtime-zoom-xHaMHl/receipt.json", SHA: "ef85abc90d63e79ed4b47d1ce1920d0870862308cd755e6252c542b77311c533", Images: 52},
			{ID: "navbar-delivery", Gate: "integration", Root: "delivery-run", Path: "receipt.json", SHA: "bc8190f608f7d53d172650481cf7ebce34b5390f318ca4c110c1151af131b7a0", Images: 8},
		},
	}
}
