package designreceipts_test

// menuOverlaySpec retains the final, source-free P07 proof inputs and their
// historical receipts. The selected adapter sources are the current transformer
// snapshot, not a recovered historical execution closure. The older base-browser
// and baseline-zoom harness bytes are no longer present at maintained paths;
// their receipts and captures remain evidence without pretending otherwise.
func menuOverlaySpec() overlaySpec {
	return overlaySpec{
		Name:              "menu",
		Subject:           "reference.menu-adapter",
		Member:            "html/elements/dropdowns/01-simple.html",
		SourceSHA:         "10ed82d0c337ad246f1ab8687d73cc0a7dc5d834bb7b3ec46a01bc7c5d5f144a",
		ArtifactSHA:       "7917bf4253fe81b0ef97da581a39a92cc834a024b21c59656e5b36385f0a3ea0",
		InteractionReason: "The baseline actual-200% run records two failures; scoped explicit-repair passes do not establish full interaction, lifecycle, accessibility, performance, consumer acceptance, or target certification.",
		Assets: []overlayAsset{
			{ID: "menu-source", Group: "source", Root: "menu", Path: "original.html", SHA: "10ed82d0c337ad246f1ab8687d73cc0a7dc5d834bb7b3ec46a01bc7c5d5f144a"},
			{ID: "menu-adapter", Group: "transformer", Root: "kit", Path: "internal/designadapt/menu.go", SHA: "710b551a22ee5238d25f12bc821e9bf81802e48d451dd0952b03e9cd0be389c8"},
			{ID: "menu-adapter-rule", Group: "transformer", Root: "kit", Path: "internal/designadapt/menu.cue", SHA: "fade6feab2e50531291970c82765259e70705773e1fb340b166d10acb4f24650"},
			{ID: "menu-base-artifact", Group: "output", Root: "menu", Path: "delivery/artifact.json", SHA: "7917bf4253fe81b0ef97da581a39a92cc834a024b21c59656e5b36385f0a3ea0"},
			{ID: "menu-template", Group: "output", Root: "menu", Path: "delivery/menu.templ", SHA: "336e61c82c3dfec467afa64013499f255becc98c452ca9e1d8345946ef4b56c3"},
			{ID: "menu-model", Group: "output", Root: "menu", Path: "delivery/menu.go", SHA: "2b8b008eeb76824915e79d4db402f8478466965a5679dbdab094655e87b6d639"},
			{ID: "menu-enhanced-css", Group: "output", Root: "menu", Path: "delivery/menu.css", SHA: "3aca3fc896b42ae4d0ee9bbfafbba5b434e061837cedab884fd3d7568a5d957b"},
			{ID: "menu-defaults", Group: "output", Root: "menu", Path: "delivery/defaults.json", SHA: "132ebe2fc7128e4751612b997d7eae1d6cd3c788f09a170bc5e60a9339adc05e"},
			{ID: "menu-source-css", Group: "baseline", Root: "menu", Path: "browser-KTBdJj/source.css", SHA: "9686875c6ef93d743d03abaa955c2ce72975c1d21291528f8e60e9ea02b12acb"},
			{ID: "menu-repair-artifact", Group: "output", Root: "menu-repair", Path: "delivery/artifact.json", SHA: "9d18b4403ec554c1f1f40f76776dba730f5d9a8a16f0bb4228c9ab2d80754d5e"},
			{ID: "menu-repair-module", Group: "patch", Root: "menu-repair", Path: "delivery/menu_repair.mjs", SHA: "3de94e91ab8e79a84de18780d63c429d99c02c85f6b898b90d0b9bd5eec47935"},
			{ID: "menu-repair-css", Group: "patch", Root: "menu-repair", Path: "delivery/menu_repair.css", SHA: "2f98b4af12b271d74b1ec551dabb40e243ab82395e1c0c5f8e657c94118c2cd6"},
			{ID: "menu-browser-harness", Group: "fixture", Root: "kit", Path: "docs/assessments/fixtures/menu-adapter/browser.mjs", SHA: "7f0279fbbbf63f2fd7c79dca8c119589f345830cd2cd977f7dc68d74532e4f90"},
			{ID: "menu-zoom-harness", Group: "fixture", Root: "kit", Path: "docs/assessments/fixtures/menu-adapter/zoom.mjs", SHA: "eface789339149c3299740f8a691949f9db52fb254b2ecee0f2182697a0befc7"},
			{ID: "menu-compile-receipt", Group: "toolchain", Root: "menu", Path: "compile-receipt.json", SHA: "90258d4ab9d513c43eb548ae057b2cfeb8de58e70d59c6b339fbf706e62081cc"},
			{ID: "menu-elements-runtime", Group: "runtime", Root: "runtime", Path: "elements.js", SHA: "120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013"},
			{ID: "menu-inter-font", Group: "runtime", Root: "runtime", Path: "inter.woff2", SHA: "693b77d4f32ee9b8bfc995589b5fad5e99adf2832738661f5402f9978429a8e3"},
		},
		Runs: []overlayRun{
			{ID: "menu-base", Gate: "visual", Root: "menu", Path: "browser-joAWxO/receipt.json", SHA: "83ee58395928f68d0114907e522123bef9839e21f9a7630de6cb12a88be36bf0", Images: 20},
			{ID: "menu-baseline-zoom", Gate: "interaction", Root: "menu", Path: "zoom-hNfyMj/receipt.json", SHA: "7cdfe1fd07f8487c5e22ff4a60f3dad8ad566205800a31ee3229e58efd59f1ea", Images: 2},
			{ID: "menu-repair", Gate: "interaction", Root: "menu", Path: "browser-KTBdJj/receipt.json", SHA: "bae1c06dfb6f8cb3ee07e667c9a540877e6e02a82be03d790eff8a41a0f3b18d", Images: 20},
			{ID: "menu-repair-zoom", Gate: "lifecycle", Root: "menu", Path: "zoom-JdhTxt/receipt.json", SHA: "4c23f608878410158522afef51132cd2e2eec13b333324fe02a9f05365321554", Images: 10},
		},
	}
}
