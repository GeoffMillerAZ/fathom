package main

// langoracle_svelte.go — Svelte components' L1 to L3 under W4 (design §1.8
// "W4 for Vue and Svelte"; wave lf7, item vuesvelte, K19), measured against
// svelte/compiler parse (calibration/oracles/svelte-compiler, tier A), by the
// rules langoracle_vue.go documents for Vue.

func init() {
	fileResolveLang["svelte"] = "svelte"
	ownCallerRules["svelte"] = componentOwnCallers
}
