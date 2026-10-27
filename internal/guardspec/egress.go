package guardspec

// egress.go — the shipped egress pack (ADR-083): typed checks over
// internal/shellcmd for the commands that send data off this machine, plus
// the host check for a harness's own web tools. It is a tripwire, not
// data-loss prevention: it reads literal argv words only, so `python -c`,
// a script, or an encoded payload passes unseen. A target the guard cannot
// read (a shell expansion, an unresolvable git remote) is not on the
// allowlist, so it matches like any other unlisted host.

import (
	_ "embed"
	"fmt"
	"net"
	"net/url"
	"path"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

//go:embed egress.cue
var egressData []byte

const (
	egressPackName    = "egress"
	gitSafetyPackName = "git-safety"
)

// EgressOptions is what the egress pack needs from its caller; loading it
// reads no configuration and runs nothing.
type EgressOptions struct {
	// Allow are the hosts the guard lets through ([guards] egress_allow):
	// an exact host, "*.example.com" (or ".example.com") for its
	// subdomains, or "s3://bucket" / "gs://bucket" for a storage bucket.
	// Loopback hosts always pass.
	Allow []string
	// RemoteURLs resolves a git remote name to its push URLs in the
	// repository at dir (a `git -C` path, "" for the command's own
	// directory); remote "" is the default push remote. ok false leaves
	// the target unknown; ok with no URLs means the push has nowhere to go
	// (no such remote, or no repository), so nothing leaves.
	RemoteURLs func(dir, remote string) ([]string, bool)
}

var egressChecks = map[string]bool{
	"curl_upload": true, "wget_upload": true, "socket_tool": true, "remote_copy": true,
	"ssh_remote": true, "git_push_remote": true, "gh_gist_create": true, "cloud_upload": true,
	"web_fetch": true, "web_search": true,
}

// EgressPack loads the shipped egress guards from embedded data, bound to
// opts. Callers match it with MatchCommandGuard (shell commands) or
// MatchEgressWeb (a harness's web tools) and pass a matched guard to
// EvaluateCommandEnforcement when the operator chose enforce.
func EgressPack(opts EgressOptions) (GuardSet, error) {
	v := cuecontext.New().CompileBytes(egressData, cue.Filename(egressPackName))
	if err := v.Err(); err != nil {
		return GuardSet{}, err
	}
	gs, err := decodeGuards(egressPackName, v)
	if err != nil {
		return GuardSet{}, err
	}
	var data struct {
		Guards []struct {
			Check string `json:"command_check"`
		} `json:"guards"`
	}
	if err := v.Decode(&data); err != nil {
		return GuardSet{}, err
	}
	for i, g := range data.Guards {
		if !egressChecks[g.Check] {
			return GuardSet{}, fmt.Errorf("egress: unknown command check %q", g.Check)
		}
		gs.Guards[i].commandCheck = g.Check
		gs.Guards[i].pack = egressPackName
	}
	allow := make([]string, 0, len(opts.Allow))
	for _, a := range opts.Allow {
		if a = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(a), "/")); a != "" {
			allow = append(allow, a)
		}
	}
	opts.Allow = allow
	gs.egress = &opts
	return gs, nil
}

// MatchEgressWeb matches a harness web tool call against the pack: WebFetch
// by its URL's host, WebSearch by its allowed domains (a search with none
// may return any domain). Any other tool never matches.
func MatchEgressWeb(gs GuardSet, tool, rawURL string, domains []string) (Guard, bool) {
	check := map[string]string{"WebFetch": "web_fetch", "WebSearch": "web_search"}[tool]
	if check == "" {
		return Guard{}, false
	}
	opts := gs.egressOptions()
	for _, g := range gs.Guards {
		if g.pack != egressPackName || g.commandCheck != check {
			continue
		}
		var targets []egressTarget
		if check == "web_fetch" {
			targets = []egressTarget{urlTarget(shellcmd.Word{Text: rawURL})}
		} else {
			for _, d := range domains {
				targets = append(targets, urlTarget(shellcmd.Word{Text: d}))
			}
		}
		return egressVerdict(g, targets, opts.Allow)
	}
	return Guard{}, false
}

func (gs GuardSet) egressOptions() EgressOptions {
	if gs.egress == nil {
		return EgressOptions{}
	}
	return *gs.egress
}

// egressTarget is one destination a command names: a host, a bucket
// ("s3://bucket"), or unknown when the guard cannot read it.
type egressTarget struct {
	host    string
	unknown bool
}

var localTarget = egressTarget{host: "localhost"}

// matchEgressGuard runs g's predicate over one simple command.
func matchEgressGuard(g Guard, u *shellcmd.Unit, opts EgressOptions) (Guard, bool) {
	if u.Kind != shellcmd.KindSimple || len(u.Words) == 0 || !u.Words[0].Literal() {
		return Guard{}, false
	}
	var targets []egressTarget
	var hit bool
	switch g.commandCheck {
	case "curl_upload":
		targets, hit = curlUpload(u)
	case "wget_upload":
		targets, hit = wgetUpload(u)
	case "socket_tool":
		targets, hit = socketTool(u)
	case "remote_copy":
		targets, hit = remoteCopy(u)
	case "ssh_remote":
		targets, hit = sshRemote(u)
	case "git_push_remote":
		targets, hit = gitPushRemote(u, opts)
	case "gh_gist_create":
		targets, hit = ghGistCreate(u)
	case "cloud_upload":
		targets, hit = cloudUpload(u)
	}
	if !hit {
		return Guard{}, false
	}
	return egressVerdict(g, targets, opts.Allow)
}

// egressVerdict matches when any target is unknown or off the allowlist,
// and words the note for the first such target.
func egressVerdict(g Guard, targets []egressTarget, allow []string) (Guard, bool) {
	if len(targets) == 0 {
		targets = []egressTarget{{unknown: true}}
	}
	for _, t := range targets {
		if t.unknown || !egressAllowed(t.host, allow) {
			g.Note = egressNote(g, t)
			return g, true
		}
	}
	return Guard{}, false
}

// egressNote is the one advisory line: the rule, what it saw, and the
// allowlist entry that would let the call through.
func egressNote(g Guard, t egressTarget) string {
	const off = `; [guards] egress = "off" turns the egress guard off.`
	rule := egressPackName + "/" + g.commandCheck
	if t.unknown {
		if g.commandCheck == "web_search" {
			return fmt.Sprintf("%s: %s, with results from any domain. To allow it, restrict allowed_domains to hosts in [guards] egress_allow (~/.fathom/config.toml)%s", rule, g.Note, off)
		}
		return fmt.Sprintf("%s: %s to a host this guard cannot read (a shell expansion, or no literal target). To allow it, name the host literally and add it to [guards] egress_allow in ~/.fathom/config.toml%s", rule, g.Note, off)
	}
	return fmt.Sprintf("%s: %s to %s. To allow it, add %q to [guards] egress_allow in ~/.fathom/config.toml%s", rule, g.Note, t.host, t.host, off)
}

// egressAllowed reports whether target is loopback or on the allowlist.
func egressAllowed(target string, allow []string) bool {
	t := strings.ToLower(strings.TrimSuffix(target, "."))
	if egressLoopback(t) {
		return true
	}
	bucket := strings.Contains(t, "://")
	for _, a := range allow {
		switch {
		case strings.Contains(a, "://"):
			if t == a {
				return true
			}
		case bucket:
		case strings.HasPrefix(a, "*."):
			if strings.HasSuffix(t, a[1:]) {
				return true
			}
		case strings.HasPrefix(a, "."):
			if strings.HasSuffix(t, a) {
				return true
			}
		case t == a:
			return true
		}
	}
	return false
}

func egressLoopback(host string) bool {
	h := strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	if h == "localhost" || strings.HasSuffix(h, ".localhost") || h == "0.0.0.0" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// urlTarget reads a URL, or a bare host[:port][/path], as a target. A
// file URL is local; a storage URL is its bucket.
func urlTarget(w shellcmd.Word) egressTarget {
	if !w.Literal() || strings.TrimSpace(w.Text) == "" {
		return egressTarget{unknown: true}
	}
	s := w.Text
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return egressTarget{unknown: true}
	}
	switch scheme := strings.ToLower(u.Scheme); scheme {
	case "file":
		return localTarget
	case "s3", "gs":
		return egressTarget{host: scheme + "://" + strings.ToLower(u.Host)}
	}
	if u.Hostname() == "" {
		return egressTarget{unknown: true}
	}
	return egressTarget{host: strings.ToLower(u.Hostname())}
}

// ─── argv reading ───────────────────────────────────────────────────────

// egressSpec names a tool's value-taking options: short letters, and long
// names that take the next word when written without "=".
type egressSpec struct {
	short string
	long  map[string]bool
}

type egressOpt struct {
	name  string
	value shellcmd.Word
	has   bool
}

func longSet(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

// readEgressArgs splits words into options and positional words. An
// option's name is read from the word's text even when its value needs
// expansion; a word whose name itself needs expansion is positional.
func readEgressArgs(words []shellcmd.Word, spec egressSpec) (opts []egressOpt, pos []shellcmd.Word) {
	for i := 0; i < len(words); i++ {
		w := words[i]
		t := w.Text
		if !strings.HasPrefix(t, "-") || t == "-" {
			pos = append(pos, w)
			continue
		}
		if w.Literal() && t == "--" {
			return opts, append(pos, words[i+1:]...)
		}
		if strings.HasPrefix(t, "--") {
			name, val, eq := strings.Cut(t, "=")
			switch {
			case strings.ContainsAny(name, "$`*?[{"):
				pos = append(pos, w)
			case eq:
				opts = append(opts, egressOpt{name: name, value: shellcmd.Word{Text: val, Dynamic: w.Dynamic, Glob: w.Glob}, has: true})
			case spec.long[name] && i+1 < len(words):
				i++
				opts = append(opts, egressOpt{name: name, value: words[i], has: true})
			default:
				opts = append(opts, egressOpt{name: name})
			}
			continue
		}
		for j := 1; j < len(t); j++ {
			c := t[j]
			if !isOptLetter(c) {
				break
			}
			name := "-" + string(c)
			if strings.IndexByte(spec.short, c) < 0 {
				opts = append(opts, egressOpt{name: name})
				continue
			}
			switch {
			case j+1 < len(t):
				opts = append(opts, egressOpt{name: name, value: shellcmd.Word{Text: t[j+1:], Dynamic: w.Dynamic, Glob: w.Glob}, has: true})
			case i+1 < len(words):
				i++
				opts = append(opts, egressOpt{name: name, value: words[i], has: true})
			default:
				opts = append(opts, egressOpt{name: name})
			}
			break
		}
	}
	return opts, pos
}

func isOptLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '#' || c == ':'
}

func hasOpt(opts []egressOpt, names ...string) bool {
	for _, o := range opts {
		for _, n := range names {
			if o.name == n {
				return true
			}
		}
	}
	return false
}

func writeMethod(m string) bool {
	switch strings.ToUpper(m) {
	case "POST", "PUT", "PATCH":
		return true
	}
	return false
}

// ─── checks ─────────────────────────────────────────────────────────────

var curlSpec = egressSpec{short: "AbcCdDeEFHKmoPQrtTuUwxXyYz", long: longSet(
	"--data", "--data-ascii", "--data-binary", "--data-raw", "--data-urlencode", "--json",
	"--form", "--form-string", "--upload-file", "--request", "--url", "--header", "--output",
	"--user", "--user-agent", "--referer", "--cookie", "--cookie-jar", "--proxy", "--max-time",
	"--connect-timeout", "--write-out", "--config", "--range", "--cert", "--key", "--cacert",
	"--capath", "--resolve", "--connect-to", "--retry", "--retry-delay", "--retry-max-time",
	"--continue-at", "--limit-rate", "--interface", "--dns-servers", "--proxy-user",
	"--oauth2-bearer", "--aws-sigv4", "--variable", "--dump-header", "--trace", "--trace-ascii",
	"--stderr", "--output-dir", "--unix-socket", "--abstract-unix-socket", "--max-filesize",
	"--max-redirs", "--noproxy", "--pass", "--preproxy", "--proxy-header", "--quote",
	"--request-target", "--socks4", "--socks4a", "--socks5", "--socks5-hostname", "--speed-limit",
	"--speed-time", "--time-cond", "--url-query", "--mail-from", "--mail-rcpt", "--ciphers",
	"--cert-type", "--key-type", "--ftp-port", "--doh-url", "--local-port", "--expect100-timeout",
	"--keepalive-time", "--hostpubmd5", "--hostpubsha256", "--pinnedpubkey", "--tlsuser",
	"--tlspassword", "--sasl-authzid", "--service-name", "--login-options", "--mail-auth",
	"--engine", "--delegation", "--krb", "--telnet-option",
)}

func curlUpload(u *shellcmd.Unit) ([]egressTarget, bool) {
	if u.Name() != "curl" {
		return nil, false
	}
	opts, pos := readEgressArgs(u.Words[1:], curlSpec)
	upload := false
	for _, o := range opts {
		switch {
		case o.name == "-d" || strings.HasPrefix(o.name, "--data") || o.name == "--json",
			o.name == "-F" || o.name == "--form" || o.name == "--form-string",
			o.name == "-T" || o.name == "--upload-file":
			upload = true
		case o.name == "-X" || o.name == "--request":
			upload = upload || o.has && o.value.Literal() && writeMethod(o.value.Text)
		case o.name == "--url" && o.has:
			pos = append(pos, o.value)
		}
	}
	if !upload {
		return nil, false
	}
	return wordTargets(pos), true
}

var wgetSpec = egressSpec{short: "oaeiBtTwQPUlARDIXOn", long: longSet(
	"--output-document", "--output-file", "--append-output", "--execute", "--input-file", "--base",
	"--tries", "--timeout", "--wait", "--quota", "--directory-prefix", "--user-agent", "--level",
	"--accept", "--reject", "--domains", "--exclude-domains", "--include-directories",
	"--exclude-directories", "--header", "--user", "--password", "--http-user", "--http-password",
	"--post-data", "--post-file", "--body-data", "--body-file", "--method", "--referer",
	"--load-cookies", "--save-cookies", "--certificate", "--private-key", "--ca-certificate",
	"--ca-directory", "--bind-address", "--limit-rate", "--proxy-user", "--proxy-password",
	"--cut-dirs", "--default-page", "--local-encoding", "--remote-encoding", "--rejected-log",
	"--waitretry", "--dns-timeout", "--connect-timeout", "--read-timeout", "--restrict-file-names",
	"--progress", "--secure-protocol", "--ciphers", "--config", "--use-askpass", "--accept-regex",
	"--reject-regex", "--regex-type", "--follow-tags", "--ignore-tags", "--max-redirect",
)}

func wgetUpload(u *shellcmd.Unit) ([]egressTarget, bool) {
	if u.Name() != "wget" {
		return nil, false
	}
	opts, pos := readEgressArgs(u.Words[1:], wgetSpec)
	upload := false
	for _, o := range opts {
		switch o.name {
		case "--post-data", "--post-file", "--body-data", "--body-file":
			upload = true
		case "--method":
			upload = upload || o.has && o.value.Literal() && writeMethod(o.value.Text)
		}
	}
	if !upload {
		return nil, false
	}
	return wordTargets(pos), true
}

func wordTargets(words []shellcmd.Word) []egressTarget {
	out := make([]egressTarget, 0, len(words))
	for _, w := range words {
		out = append(out, urlTarget(w))
	}
	return out
}

var netcatSpec = egressSpec{short: "cegGiIOpsTVwxXPMmq", long: longSet(
	"--exec", "--sh-exec", "--lua-exec", "--source", "--source-port", "--proxy", "--proxy-type",
	"--proxy-auth", "--proxy-dns", "--wait", "--idle-timeout", "--output", "--hex-dump", "--allow",
	"--allowfile", "--deny", "--denyfile", "--max-conns", "--delay", "--ssl-cert", "--ssl-key",
	"--ssl-trustfile", "--ssl-ciphers", "--ssl-servername", "--ssl-alpn",
)}

// socatNetwork are the socat address types that connect to a host; a
// LISTEN or RECV type accepts from anyone, so its peer is unknown.
var socatNetwork = []string{"TCP", "UDP", "OPENSSL", "SSL", "SCTP", "SOCKS", "PROXY", "DCCP", "IP-SENDTO", "IP4-SENDTO", "IP6-SENDTO", "IP-DATAGRAM", "IP4-DATAGRAM", "IP6-DATAGRAM", "VSOCK"}

func socketTool(u *shellcmd.Unit) ([]egressTarget, bool) {
	switch u.Name() {
	case "nc", "ncat", "netcat":
		opts, pos := readEgressArgs(u.Words[1:], netcatSpec)
		switch {
		case hasOpt(opts, "-U", "--unixsock"):
			return nil, false
		case hasOpt(opts, "-l", "--listen"):
			return []egressTarget{{unknown: true}}, true
		case len(pos) == 0:
			return nil, false
		}
		return []egressTarget{hostTarget(pos[0])}, true
	case "socat":
		_, pos := readEgressArgs(u.Words[1:], egressSpec{short: "btTLWrRS"})
		var targets []egressTarget
		for _, w := range pos {
			if !w.Literal() {
				targets = append(targets, egressTarget{unknown: true})
				continue
			}
			if t, ok := socatAddress(w.Text); ok {
				targets = append(targets, t)
			}
		}
		return targets, len(targets) > 0
	}
	return nil, false
}

func socatAddress(addr string) (egressTarget, bool) {
	typ, rest, _ := strings.Cut(addr, ":")
	typ, _, _ = strings.Cut(typ, ",")
	typ = strings.ToUpper(typ)
	if strings.Contains(typ, "LISTEN") || strings.Contains(typ, "RECV") {
		return egressTarget{unknown: true}, true
	}
	network := false
	for _, p := range socatNetwork {
		network = network || strings.HasPrefix(typ, p)
	}
	if !network {
		return egressTarget{}, false
	}
	host := rest
	if strings.HasPrefix(host, "[") {
		if end := strings.Index(host, "]"); end > 0 {
			host = host[1:end]
		}
	} else if i := strings.IndexAny(host, ":,"); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return egressTarget{unknown: true}, true
	}
	return egressTarget{host: strings.ToLower(host)}, true
}

// hostTarget reads a bare host word ([user@]host).
func hostTarget(w shellcmd.Word) egressTarget {
	if !w.Literal() || w.Text == "" {
		return egressTarget{unknown: true}
	}
	h := w.Text
	if i := strings.LastIndex(h, "@"); i >= 0 {
		h = h[i+1:]
	}
	return egressTarget{host: strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(h, "["), "]"))}
}

// remoteSpec reads an scp/rsync/sftp location: a URL, [user@]host:path,
// host::module, or a local path (remote false).
func remoteSpec(w shellcmd.Word) (egressTarget, bool) {
	if !w.Literal() {
		return egressTarget{unknown: true}, true
	}
	t := w.Text
	if i := strings.Index(t, "://"); i > 0 && !strings.ContainsAny(t[:i], "/:") {
		target := urlTarget(w)
		return target, target != localTarget
	}
	if i := strings.LastIndex(t, "@"); i >= 0 && !strings.Contains(t[:i], "/") {
		t = t[i+1:]
	}
	var host string
	if strings.HasPrefix(t, "[") {
		end := strings.Index(t, "]")
		if end < 0 || !strings.HasPrefix(t[end+1:], ":") {
			return egressTarget{}, false
		}
		host = t[1:end]
	} else {
		colon := strings.Index(t, ":")
		if colon <= 0 || strings.Contains(t[:colon], "/") {
			return egressTarget{}, false
		}
		host = t[:colon]
	}
	return egressTarget{host: strings.ToLower(host)}, true
}

var (
	scpSpec   = egressSpec{short: "cDFiJloPSX"}
	sftpSpec  = egressSpec{short: "BbcDFiJloPRSsX"}
	rsyncSpec = egressSpec{short: "efTMB", long: longSet(
		"--rsh", "--rsync-path", "--exclude", "--include", "--filter", "--exclude-from",
		"--include-from", "--files-from", "--password-file", "--port", "--log-file",
		"--log-file-format", "--backup-dir", "--suffix", "--chmod", "--chown", "--usermap",
		"--groupmap", "--timeout", "--contimeout", "--bwlimit", "--compare-dest", "--copy-dest",
		"--link-dest", "--partial-dir", "--temp-dir", "--max-size", "--min-size", "--max-delete",
		"--max-alloc", "--out-format", "--info", "--debug", "--remote-option", "--iconv",
		"--protocol", "--sockopts", "--checksum-choice", "--compress-choice", "--compress-level",
		"--skip-compress", "--modify-window", "--block-size", "--address", "--outbuf",
		"--stop-after", "--stop-at", "--write-batch", "--read-batch", "--only-write-batch",
	)}
)

// remoteCopy flags scp and rsync by their destination (the last location;
// a download sends nothing) and sftp by its host.
func remoteCopy(u *shellcmd.Unit) ([]egressTarget, bool) {
	switch u.Name() {
	case "scp", "rsync":
		spec := scpSpec
		if u.Name() == "rsync" {
			spec = rsyncSpec
		}
		_, pos := readEgressArgs(u.Words[1:], spec)
		if len(pos) < 2 {
			return nil, false
		}
		t, remote := remoteSpec(pos[len(pos)-1])
		return []egressTarget{t}, remote
	case "sftp":
		_, pos := readEgressArgs(u.Words[1:], sftpSpec)
		if len(pos) == 0 {
			return nil, false
		}
		w := pos[0]
		if t, remote := remoteSpec(w); remote {
			return []egressTarget{t}, true
		}
		return []egressTarget{hostTarget(w)}, true
	}
	return nil, false
}

var sshSpec = egressSpec{short: "BbcDEeFIiJLlmOoPpQRSWw"}

func sshRemote(u *shellcmd.Unit) ([]egressTarget, bool) {
	if u.Name() != "ssh" {
		return nil, false
	}
	opts, pos := readEgressArgs(u.Words[1:], sshSpec)
	if len(pos) == 0 || hasOpt(opts, "-G", "-V", "-Q") {
		return nil, false
	}
	w := pos[0]
	if w.Literal() && strings.Contains(w.Text, "://") {
		return []egressTarget{urlTarget(w)}, true
	}
	return []egressTarget{hostTarget(w)}, true
}

// gitPushRemote reads `git [-C dir] push [options] [repository]`. A
// repository URL is read as written; a remote name, or none, is resolved
// through opts.RemoteURLs. A dry run sends nothing.
func gitPushRemote(u *shellcmd.Unit, opts EgressOptions) ([]egressTarget, bool) {
	if u.Name() != "git" {
		return nil, false
	}
	words := u.Words[1:]
	dir, dirKnown := "", true
	i := 0
global:
	for ; i < len(words); i++ {
		w := words[i]
		if !w.Literal() {
			return nil, false
		}
		switch t := w.Text; {
		case t == "-C":
			if i+1 >= len(words) || !words[i+1].Literal() {
				dirKnown = false
			} else {
				dir = path.Join(dir, words[i+1].Text)
				if path.IsAbs(words[i+1].Text) {
					dir = words[i+1].Text
				}
			}
			i++
		case t == "-c" || t == "--namespace" || t == "--config-env" || t == "--exec-path":
			i++
		case t == "--git-dir" || t == "--work-tree":
			dirKnown = false
			i++
		case strings.HasPrefix(t, "--git-dir=") || strings.HasPrefix(t, "--work-tree="):
			dirKnown = false
		case strings.HasPrefix(t, "-"):
		default:
			break global
		}
	}
	if i >= len(words) || words[i].Text != "push" {
		return nil, false
	}
	var repo, repoOpt *shellcmd.Word
	for j := i + 1; j < len(words); j++ {
		w := words[j]
		t := w.Text
		if w.Literal() && t == "--" {
			if repo == nil && j+1 < len(words) {
				repo = &words[j+1]
			}
			break
		}
		if w.Literal() && strings.HasPrefix(t, "-") {
			key, val, eq := strings.Cut(t, "=")
			switch key = safetyGitLongOption("push", key); key {
			case "--dry-run":
				return nil, false
			case "--repo":
				if eq {
					repoOpt = &shellcmd.Word{Text: val}
				} else if j+1 < len(words) {
					j++
					repoOpt = &words[j]
				}
			case "--receive-pack", "--exec", "--push-option", "-o":
				if !eq {
					j++
				}
			default:
				if !strings.HasPrefix(t, "--") && strings.ContainsRune(t[1:], 'n') {
					return nil, false
				}
			}
			continue
		}
		if repo == nil {
			repo = &words[j]
		}
	}
	if repo == nil {
		repo = repoOpt
	}
	var urls []string
	switch {
	case repo != nil && !repo.Literal():
		return []egressTarget{{unknown: true}}, true
	case repo != nil && gitLocalPath(repo.Text):
		return nil, false
	case repo != nil && gitURLLike(repo.Text):
		urls = []string{repo.Text}
	default:
		name := ""
		if repo != nil {
			name = repo.Text
		}
		ok := false
		if dirKnown && opts.RemoteURLs != nil {
			urls, ok = opts.RemoteURLs(dir, name)
		}
		if !ok {
			return []egressTarget{{unknown: true}}, true
		}
	}
	targets := make([]egressTarget, 0, len(urls))
	for _, raw := range urls {
		if gitLocalPath(raw) {
			continue
		}
		if !gitURLLike(raw) {
			targets = append(targets, egressTarget{unknown: true})
			continue
		}
		if t, remote := remoteSpec(shellcmd.Word{Text: raw}); remote {
			targets = append(targets, t)
		}
	}
	return targets, len(targets) > 0
}

func gitLocalPath(s string) bool {
	return s == "." || s == ".." || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") ||
		strings.HasPrefix(s, "../") || strings.HasPrefix(s, "~") || strings.HasPrefix(strings.ToLower(s), "file://")
}

// gitURLLike reports a URL or an scp-like [user@]host:path, never a
// remote name (which carries no colon).
func gitURLLike(s string) bool {
	if strings.Contains(s, "://") {
		return true
	}
	colon := strings.Index(s, ":")
	return colon > 0 && !strings.Contains(s[:colon], "/")
}

func ghGistCreate(u *shellcmd.Unit) ([]egressTarget, bool) {
	if u.Name() != "gh" || len(u.Words) < 3 || u.Words[1].Text != "gist" {
		return nil, false
	}
	if sub := u.Words[2].Text; sub != "create" && sub != "new" {
		return nil, false
	}
	host := "gist.github.com"
	for _, a := range u.Env {
		if a.Name == "GH_HOST" {
			if !a.Value.Literal() {
				return []egressTarget{{unknown: true}}, true
			}
			host = strings.ToLower(a.Value.Text)
		}
	}
	return []egressTarget{{host: host}}, true
}

var (
	awsGlobalSpec = egressSpec{long: longSet("--profile", "--region", "--endpoint-url", "--output", "--query", "--color", "--cli-read-timeout", "--cli-connect-timeout", "--ca-bundle", "--cli-binary-format")}
	awsS3Spec     = egressSpec{long: longSet(
		"--profile", "--region", "--endpoint-url", "--output", "--query", "--color", "--cli-read-timeout",
		"--cli-connect-timeout", "--ca-bundle", "--exclude", "--include", "--acl", "--grants", "--sse",
		"--sse-c", "--sse-c-key", "--sse-kms-key-id", "--sse-c-copy-source", "--sse-c-copy-source-key",
		"--storage-class", "--website-redirect", "--content-type", "--cache-control",
		"--content-disposition", "--content-encoding", "--content-language", "--expires",
		"--source-region", "--metadata", "--metadata-directive", "--expected-size", "--request-payer",
		"--page-size", "--checksum-mode", "--checksum-algorithm", "--copy-props",
	)}
	gsutilGlobalSpec = egressSpec{short: "uohi"}
	gsutilSubSpec    = egressSpec{short: "aLzjsx"}
	gcloudSpec       = egressSpec{short: "x", long: longSet(
		"--project", "--account", "--configuration", "--billing-project", "--impersonate-service-account",
		"--verbosity", "--format", "--flags-file", "--flatten", "--trace-token", "--access-token-file",
		"--storage-class", "--content-type", "--cache-control", "--content-disposition",
		"--content-encoding", "--content-language", "--custom-time", "--predefined-acl",
		"--encryption-key", "--decryption-keys", "--additional-headers", "--custom-metadata",
		"--exclude", "--manifest-path",
	)}
)

// cloudUpload flags a storage copy whose destination (the last location)
// is a bucket, and every az storage blob upload.
func cloudUpload(u *shellcmd.Unit) ([]egressTarget, bool) {
	switch u.Name() {
	case "aws":
		_, pos := readEgressArgs(u.Words[1:], awsGlobalSpec)
		if len(pos) < 2 || pos[0].Text != "s3" {
			return nil, false
		}
		switch pos[1].Text {
		case "cp", "sync", "mv":
		default:
			return nil, false
		}
		return bucketDestination(u.Words, awsS3Spec, "s3")
	case "gsutil":
		_, pos := readEgressArgs(u.Words[1:], gsutilGlobalSpec)
		if len(pos) == 0 {
			return nil, false
		}
		switch pos[0].Text {
		case "cp", "mv", "rsync":
		default:
			return nil, false
		}
		return bucketDestination(u.Words, gsutilSubSpec, pos[0].Text)
	case "gcloud":
		_, pos := readEgressArgs(u.Words[1:], gcloudSpec)
		for len(pos) > 0 && (pos[0].Text == "alpha" || pos[0].Text == "beta") {
			pos = pos[1:]
		}
		if len(pos) < 2 || pos[0].Text != "storage" {
			return nil, false
		}
		switch pos[1].Text {
		case "cp", "mv", "rsync":
		default:
			return nil, false
		}
		return bucketDestination(u.Words, gcloudSpec, pos[1].Text)
	case "az":
		if len(u.Words) < 4 || u.Words[1].Text != "storage" || u.Words[2].Text != "blob" {
			return nil, false
		}
		if sub := u.Words[3].Text; sub != "upload" && sub != "upload-batch" {
			return nil, false
		}
		opts, _ := readEgressArgs(u.Words[4:], egressSpec{long: longSet("--account-name")})
		for _, o := range opts {
			if o.name == "--account-name" && o.has && o.value.Literal() {
				return []egressTarget{{host: strings.ToLower(o.value.Text) + ".blob.core.windows.net"}}, true
			}
		}
		return []egressTarget{{unknown: true}}, true
	}
	return nil, false
}

// bucketDestination reads the words after verb with spec and matches when
// the last location is a storage URL.
func bucketDestination(words []shellcmd.Word, spec egressSpec, verb string) ([]egressTarget, bool) {
	start := 0
	for i, w := range words {
		if i > 0 && w.Text == verb {
			start = i + 1
			break
		}
	}
	_, pos := readEgressArgs(words[start:], spec)
	if len(pos) < 2 {
		return nil, false
	}
	dst := pos[len(pos)-1]
	if !dst.Literal() {
		return []egressTarget{{unknown: true}}, true
	}
	lower := strings.ToLower(dst.Text)
	if !strings.HasPrefix(lower, "s3://") && !strings.HasPrefix(lower, "gs://") {
		return nil, false
	}
	return []egressTarget{urlTarget(dst)}, true
}
