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

// Work-intelligence zones by path glob (WI-1).
exampleZones: #Config & {
	zones: {
		payments: ["internal/pay/**", "cmd/payd/*"]
		console: ["internal/console/**"]
	}
}
