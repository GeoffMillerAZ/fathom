package shellbudget

// contract is deliberately concrete: the runner exports and freezes this exact
// value before it starts the supplied Gallery binary.
contract: {
	version: "shell-budget-v3"
	inputs: {
		gallerySHA256: "001225811396ef0dd2b525cf9d5ab17f4274f7fb0c0036e777ca94888c2b6631"
		cssSHA256:     "0bb39edec5eb03cfe1a79ddf9ca1559399b56b3312e550b5782b115423b19ff8"
		shellDemoJS: {sha256: "1c9a3096767c4864f05fcf861957ca7a2ea6c0584cb7f7001fa5ce4ddbfec6a4", bytes: 4487}
	}
	budget: {
		decodedResponseBodyBytes: 131072
		responseEndMs:            100
		domContentLoadedMs:       250
		loadEventMs:              500
	}
	lanes: [
		{width: 1440, height: 1000, theme: "light", javaScriptEnabled: true},
		{width: 320, height: 900, theme: "dark", javaScriptEnabled: false},
	]
	repetitions: 3
	paths: {
		profile:  "/shell/profile"
		settings: "/shell/settings"
		action:   "/shell/action"
		css:      "/static/app.css"
	}
}
