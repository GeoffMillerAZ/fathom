// Shipped egress guards (ADR-083). A tripwire for commands and web tools
// that send data off this machine, not data-loss prevention: string
// matching never sees `python -c`, a script, or an encoded payload.
// Predicates are private to this embedded pack. Each note names what the
// rule saw; the matcher adds the host and how to allow it. Loopback hosts
// and [guards] egress_allow (user scope) always pass.
guards: [{
	command_scopes: ["curl *"]
	command_check: "curl_upload"
	enforce: true
	note: "curl sends data (-d, --data*, --json, -F, -T, --upload-file or -X POST|PUT|PATCH)"
}, {
	command_scopes: ["wget *"]
	command_check: "wget_upload"
	enforce: true
	note: "wget sends data (--post-*, --body-* or --method POST|PUT|PATCH)"
}, {
	command_scopes: ["nc *", "ncat *", "netcat *", "socat *"]
	command_check: "socket_tool"
	enforce: true
	note: "a raw socket tool (nc, ncat, netcat, socat) opens a connection"
}, {
	command_scopes: ["scp *", "rsync *", "sftp *"]
	command_check: "remote_copy"
	enforce: true
	note: "a remote copy (scp, rsync, sftp) sends files"
}, {
	command_scopes: ["ssh *"]
	command_check: "ssh_remote"
	enforce: true
	note: "ssh opens a remote shell"
}, {
	command_scopes: ["git *"]
	command_check: "git_push_remote"
	enforce: true
	note: "git push sends commits"
}, {
	command_scopes: ["gh gist *"]
	command_check: "gh_gist_create"
	enforce: true
	note: "gh gist create publishes files"
}, {
	command_scopes: ["aws *", "gsutil *", "gcloud *", "az *"]
	command_check: "cloud_upload"
	enforce: true
	note: "a cloud CLI uploads files (aws s3 cp|sync|mv, gsutil, gcloud storage, az storage blob upload)"
}, {
	command_scopes: ["WebFetch"]
	command_check: "web_fetch"
	enforce: true
	note: "WebFetch sends a request"
}, {
	command_scopes: ["WebSearch"]
	command_check: "web_search"
	enforce: true
	note: "WebSearch sends its query to the search provider"
}]
