package config

// A well-formed config carrying one profile of each kind that needs
// non-trivial fields, plus a daemon table — what an operator's
// ~/.fathom/config.toml converts to after TOML→CUE.
example: #Config & {
	default_profile: "work"
	profiles: {
		work: {
			provider: "anthropic"
			model:    "claude-opus-4-1-20250805"
		}
		subscription: {
			provider: "claude-cli"
			bin:      "/opt/homebrew/bin/claude"
			timeout:  "20m"
		}
		vault: {
			provider: "openai"
			api_key_cmd: ["op", "read", "op://vault/openai/credential"]
		}
		"bedrock-govcloud": {
			provider: "bedrock"
			region:   "us-gov-west-1"
			model:    "anthropic.claude-3-5-sonnet-20241022-v2:0"
		}
		"bedrock-fips": {
			provider: "bedrock"
			region:   "us-gov-west-1"
			endpoint: "https://bedrock-runtime-fips.us-gov-west-1.amazonaws.com"
			model:    "anthropic.claude-3-5-sonnet-20241022-v2:0"
		}
		"azure-gov": {
			provider:    "azure-openai"
			endpoint:    "https://example-resource.openai.azure.us"
			deployment:  "gov-deployment"
			api_version: "2024-06-01"
			api_key_env: "AZURE_OPENAI_API_KEY"
		}
	}
	daemon: {
		store:           "workspace/db/fleet.duckdb"
		addr:            "127.0.0.1:7575"
		auto_refresh:    true
		default_profile: "work"
		memory_limit:    "3GiB"
	}
	decide: {
		provider:          "llm"
		profile:           "work"
		compile_threshold: 0.5
		route_confidence:  0.9
		rank_within_tier:  true
		sites: ["route", "obligations_rerank"]
	}
}

// A repository's own [decide] sites opting into new judgment sites (JF-4,
// judgment-foundations §4, R2): the opt-in names validate in any layer,
// but only the project layer's list turns one on (decide.SiteAllowed).
// "route" is off by default (JF-7) and comes back when named.
exampleOptInSites: #Config & {
	decide: {
		provider: "typesafe"
		sites: ["route", "compile_prefilter", "triage_failure", "ask", "explore", "compact_advice", "judge_failure"]
	}
}

// A [decide] block using the TypeSafe (Jev) adapter directly (ADR-042
// amendment, 2026-09-22): endpoint/model are optional overrides, and the
// key comes from one of api_key_env or api_key_cmd — never a literal.
exampleTypeSafe: #Config & {
	decide: {
		provider:    "typesafe"
		endpoint:    "https://api.typesafe.ai"
		model:       "jev-latest"
		api_key_env: "TYPESAFE_API_KEY"
	}
}

// A [decide] block using OpenAI's Decisions API (2026-10-09, ADR-042
// amendment): model and base_url are optional (the defaults are shown), and
// the key comes from a chmod 600 file the adapter reads at first use.
exampleOpenAI: #Config & {
	decide: {
		provider:     "openai"
		model:        "gpt-6-luna"
		base_url:     "https://api.openai.com/v1"
		transport:    "openai"
		api_key_file: "~/secrets/<provider>/token"
	}
}

// Jev over OpenRouter's Decisions router: the same System One wire, an
// OpenRouter key named by variable.
exampleTypeSafeOverOpenRouter: #Config & {
	decide: {
		provider:    "typesafe"
		transport:   "openrouter"
		model:       "typesafe/jev-1.13"
		api_key_env: "OPENROUTER_API_KEY"
	}
}

// A repository lowering the per-site, per-session ask cap (KS-10): the
// minimum across layers, so it can lower the user's and never raise it.
exampleDecideCap: #Config & {
	decide: {
		provider:                      "typesafe"
		max_asks_per_site_per_session: 50
	}
}

// The user's own consent and kill fields (KS-3, ADR-073 decision 5), as
// ~/.fathom/config.toml carries them: the port off until enabled flips
// back, a repository's provider allowed, a session site opted in and a site
// denied.
exampleDecideConsent: #Config & {
	decide: {
		provider:            "typesafe"
		enabled:             false
		allow_repo_provider: true
		session_sites: ["readout_mode", "compact_advice"]
		deny_sites: ["assess", "judge_failure"]
	}
}

// Privacy path globs (I61) whose spans never leave for a decision
// provider; every layer's globs union.
exampleDecideExclude: #Config & {
	decide: {
		provider: "typesafe"
		exclude: ["secrets/**", "*.pem"]
	}
}

// The responsible-tenant [tenancy] table (ADR-047, design §1) as an
// operator's ~/.fathom/config.toml carries it.
exampleTenancy: #Config & {
	tenancy: {
		budget:            "auto"
		shed_at:           0.75
		refuse_at:         1.0
		exit_at:           2.0
		exit_enabled:      true
		gc_hint:           "exit"
		admission:         "observe"
		gomaxprocs:        "auto"
		child_parallelism: 2
		notify:            true
		host: {
			shed_on_pressure:      "warn"
			refuse_on_pressure:    "critical"
			refuse_on_swap_growth: "2GiB"
			swap_growth_window:    "5m"
			min_swap_volume_free:  "5GiB"
			min_store_volume_free: "2GiB"
			allowance:             0.45
		}
		breaker: {
			safe_after:  3
			safe_window: "30m"
			stop_after:  5
			stop_window: "60m"
			warmup:      "120s"
		}
		unmanaged: {
			idle_exit: "20m"
			ttl:       "8h"
			budget:    "512MiB"
		}
		caches: {
			fraction:  0.25
			shed_trim: 0.5
			idle_drop: "30m"
		}
	}
}

// An explicit budget, a sized GC hint and a fixed GOMAXPROCS.
exampleTenancySized: #Config & {
	tenancy: {
		budget:     "3GiB"
		gc_hint:    "6GiB"
		gomaxprocs: 4
	}
}

// The console's conserve cap (RT-118).
exampleConsole: #Config & {
	console: conserve_max_files: 200
}

// The CI summary files /work reads CI minutes from (WI-2w): a JUnit file
// names its commit, a workflow-run export may carry its own head_sha.
exampleWork: #Config & {
	work: ci_summaries: [
		{path: "ci/junit.xml", commit: "0123abc"},
		{path: "/abs/ci/runs.json"},
	]
}

// Work-intelligence zones by path glob (WI-1).
exampleZones: #Config & {
	zones: {
		payments: ["internal/pay/**", "cmd/payd/*"]
		console: ["internal/console/**"]
	}
}

// The test-run hook's opt-in and a repository's own declared test
// commands (TR-1, TR-9; docs/plans/test-run-hook.md).
exampleTestRunHook: #Config & {
	tests: {
		commands: ["go test", "bin/itest"]
	}
	hooks: {
		test_run: true
	}
}

// The stop claim check and the gate preflight (RO-38..RO-40), both opt-in.
exampleClaimCheckAndPreflight: #Config & {
	hooks: {
		test_run:       true
		claim_check:    true
		gate_preflight: true
	}
}

// The Read outline hint (H8), opt-in.
exampleReadOutlineHint: #Config & {
	hooks: read_outline_hint: true
}

// The commit pin line's mode (PR-6, ADR-057 decision 4): off, record (the
// default when unset) or feedback.
examplePinCheck: #Config & {
	hooks: {
		pin_check: "feedback"
	}
}

// The repository URI a receipt's SARIF names in versionControlProvenance:
// an absolute URI the repository states (fathom never derives one from a
// git remote, ADR-011).
exampleReceipts: #Config & {
	receipts: repository_uri: "https://example.test/org/repo"
}

// The typed pass's nested-module exclusions (EW-68): measurement-only
// modules the repository does not want loaded.
exampleTyped: #Config & {
	typed: skip_modules: ["calibration/oracles/**", "calibration/telemetry-census"]
}

// The readout hook's modes (RO-15): user scope only, off unless set.
exampleReadouts: #Config & {
	readouts: {
		mode:  "observe"
		roots: ["/work/waves"]
		rules: "ls-long": mode: "lean"
	}
}
