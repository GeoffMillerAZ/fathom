package records
#Family: "adr" | "invariant" | "future" | "incident" | "observation" | "plan"
#Range: close({
 family: #Family
 prefix: string & =~"^[A-Z][A-Z0-9]*-?$"
 first: int & >=1
 last: int & >=first
})
#Lane: close({
 kind: "lane"
 id: string & =~"^[a-z][a-z0-9-]*$"
 goal: string & !=""
 rows: [string, ...string]
 class: "L-D" | "L-R" | "L-P" | "L-F" | "L-E" | "L-S"
 base_commit: string & =~"^[0-9a-f]{40}$"
 owned_paths: [string, ...string]
 read_only_paths: [...string]
 contract_files: [string, ...string]
 proof_command: string & !=""
 budget: string & !=""
 harness_class: string & !=""
 id_ranges: [...#Range]
})
