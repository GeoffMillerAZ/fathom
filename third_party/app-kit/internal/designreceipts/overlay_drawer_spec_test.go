package designreceipts_test

// These pins select already reviewed private runs. Current transformer files
// are separately retained snapshots, not a claim of complete historical closure.
func drawerOverlaySpec() overlaySpec {
	return overlaySpec{
		Name: "drawer", Subject: "reference.drawer-adapter", Member: "html/overlays/drawers/07-user-profile-example.html",
		SourceSHA:         "1fbde54a0be874618cc295bf3b837fee4d89f38a1543dd9a6e2374bbd1d13415",
		ArtifactSHA:       "34644520c31c87de479b4c61472b6ea3fb0e759f7c757db2fbb291fd9b2883ca",
		InteractionReason: "The unchanged base has 30 recorded actual-200% focus-policy failures. Later explicit CSS repair has scoped normal/zoom passes, not complete interaction, accessibility, lifecycle, performance or consumer certification. The repair browser runs bind base artifact plus CSS; the separate retained delivery receipt binds that CSS to the nested repair artifact.",
		Assets: []overlayAsset{
			{ID: "drawer-source", Group: "source", Root: "drawer", Path: "original.html", SHA: "1fbde54a0be874618cc295bf3b837fee4d89f38a1543dd9a6e2374bbd1d13415"},
			{ID: "drawer-base-artifact", Group: "output", Root: "drawer", Path: "delivery/artifact.json", SHA: "34644520c31c87de479b4c61472b6ea3fb0e759f7c757db2fbb291fd9b2883ca"},
			{ID: "drawer-template", Group: "output", Root: "drawer", Path: "delivery/drawer.templ", SHA: "6c7a5cbd3430df1e63e6b18a50bc9e2ea799780e6235919c905265a370d4d832"},
			{ID: "drawer-model", Group: "output", Root: "drawer", Path: "delivery/drawer.go", SHA: "6bd90ebd0d764682cc3127eb867293229e1e84fb4bd2d332d156a04a03cb962b"},
			{ID: "drawer-defaults", Group: "output", Root: "drawer", Path: "delivery/defaults.json", SHA: "46366e037cf8f81abecb8a9656a1fd1088368768491f1b0d7fdc1c84901963ae"},
			{ID: "drawer-repair-artifact", Group: "output", Root: "drawer-repair", Path: "repair-a/artifact.json", SHA: "d19692bceab358def7124107d39ec25f2512d214d41e0f4c3c78ad1690d16256"},
			{ID: "drawer-focus-css", Group: "patch", Root: "drawer-repair", Path: "repair-a/drawer_focus.css", SHA: "9b1fcd1b50999d5d82878e0444c55ad097a4755fd57df2057f2ab25bb453ffe8"},
			{ID: "drawer-source-css", Group: "baseline", Root: "drawer", Path: "browser-fXfoC6/source.css", SHA: "059627e2ba333a1999fc7941fc28a76d5e13dd7369155df95409921d88b81b2e"},
			{ID: "drawer-transformer", Group: "transformer", Root: "kit", Path: "internal/designadapt/drawer.go", SHA: "c733842df3bded2614b5f8fa6e0f0125b62be46fab3708b9decfe16ed2369855"},
			{ID: "drawer-schema", Group: "transformer", Root: "kit", Path: "internal/designadapt/drawer.cue", SHA: "62def3f95dab0d0901dfb7b5a4da495a1c64f12d92980bf218df2efea0332451"},
			{ID: "drawer-repair-transformer", Group: "transformer", Root: "kit", Path: "internal/designadapt/drawer_focus.go", SHA: "23d164cc2eb6e4c1bbb96487ea4982080f46bdb51add703d33a04d4d6208e01b"},
			{ID: "drawer-repair-schema", Group: "transformer", Root: "kit", Path: "internal/designadapt/drawer_focus.cue", SHA: "0cee7327e0e59991729f4b7b266f871547a80ae3c2d7c65094390dfcc362c040"},
			{ID: "drawer-normal-harness", Group: "fixture", Root: "kit", Path: "docs/assessments/fixtures/drawer-adapter/browser.mjs", SHA: "f939dfd904dae4920958a9f4bedfb114e5926c1fe6f24121e18186717d6b93d6"},
			{ID: "drawer-zoom-harness", Group: "fixture", Root: "kit", Path: "docs/assessments/fixtures/drawer-adapter/zoom.mjs", SHA: "ef39958d6ab627f592eebdd9db1d7d6c0d6862d71337359949805bb9ed8bedec"},
			{ID: "drawer-repair-harness", Group: "fixture", Root: "kit", Path: "docs/assessments/fixtures/drawer-adapter/repair-zoom.mjs", SHA: "824f4f35f5419ed211f106dcdca928d6118355e4ef6b2bac4925db29a5b74709"},
			{ID: "drawer-compile-receipt", Group: "toolchain", Root: "drawer", Path: "compile-receipt.json", SHA: "ca83dcba88f43e5145d672b83fe182ad49040417800f8abd1688426a0e42bc59"},
			{ID: "drawer-repair-delivery-receipt", Group: "toolchain", Root: "drawer-repair", Path: "receipt.json", SHA: "3fde6b27fd963d2109d9d6d4f64e0a718561f45c784f6d7b4bc218b981824f8d"},
			{ID: "drawer-elements-runtime", Group: "runtime", Root: "runtime", Path: "elements.js", SHA: "120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013"},
			{ID: "drawer-inter-font", Group: "runtime", Root: "runtime", Path: "inter.woff2", SHA: "693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3"},
		},
		Runs: []overlayRun{
			{ID: "drawer-base", Gate: "visual", Root: "drawer", Path: "browser-fXfoC6/receipt.json", SHA: "2d7869414fbdbfac61200ae47d9caeaec85fc96c52947718466fa73f04618c2b", Images: 24},
			{ID: "drawer-baseline-zoom", Gate: "interaction", Root: "drawer", Path: "zoom-o8SYrW/failure.json", SHA: "6c4fe2312cdc0473a2ed5f1c220db8cf4a03baa4d6e1b8b03c9d37b56a16d297", Images: 8},
			{ID: "drawer-repair", Gate: "interaction", Root: "drawer", Path: "repair-normal-MCn4as/receipt.json", SHA: "d7a60f1743214f19fd62a7e47bf4850008972d10592efd4975b87d8415dd953e", Images: 8},
			{ID: "drawer-repair-zoom", Gate: "interaction", Root: "drawer", Path: "repair-zoom-rkOoJM/receipt.json", SHA: "d0340da7e92c7cd2dd520814d81c3473c125ffe7b559eabd5b2078a7d1151ccc", Images: 4},
		},
	}
}
