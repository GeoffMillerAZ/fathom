package guardspec

// egress.go — the shipped egress pack (ADR-083): typed checks over
// internal/shellcmd for the commands that send data off this machine, plus
// the host check for a harness's own web tools. It is a tripwire, not
// data-loss prevention: it reads literal argv words only, so `python -c`,
// a script, or an encoded payload passes unseen. A target the guard cannot
// read (a shell expansion, an unresolvable git remote, a git config
// override or a GIT_*/HOME change, a curl or wget proxy, pinned address or
// config file, an ssh jump, forward, config or connection program) is not
// on the allowlist, so it matches like any other unlisted host. Input the
// lexer cannot read at all (eval, a function definition) that names an
// egress tool warns and never denies: its text cannot tell an upload from
// a read (ADR-083 decision 4e).

import (
	_ "embed"
	"fmt"
	"net"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"

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
	// directory); remote "" is the default push remote. ok false, or no
	// URLs, leaves the target unknown: a remote git cannot name now may be
	// added by an earlier step of the same command.
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
// ("s3://bucket"), or unknown when the guard cannot read it (why says
// what hid it; "" is a shell expansion or no literal target).
type egressTarget struct {
	host    string
	unknown bool
	why     string
}

// unknownTarget is a destination hidden by why.
func unknownTarget(why string) egressTarget {
	return egressTarget{unknown: true, why: why}
}

var localTarget = egressTarget{host: "localhost"}

// matchEgressGuard runs g's predicate over one simple command; prior are
// the units before it in the same command line.
func matchEgressGuard(g Guard, u *shellcmd.Unit, prior []*shellcmd.Unit, opts EgressOptions) (Guard, bool) {
	if u.Kind != shellcmd.KindSimple || len(u.Words) == 0 || !u.Words[0].Literal() {
		return Guard{}, false
	}
	var targets []egressTarget
	var hit bool
	switch g.commandCheck {
	case "curl_upload":
		targets, hit = curlUpload(u, prior)
	case "wget_upload":
		targets, hit = wgetUpload(u, prior)
	case "socket_tool":
		targets, hit = socketTool(u)
	case "remote_copy":
		targets, hit = remoteCopy(u)
	case "ssh_remote":
		targets, hit = sshRemote(u)
	case "git_push_remote":
		targets, hit = gitPushRemote(u, prior, opts)
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
		why := t.why
		if why == "" {
			why = "a shell expansion, or no literal target"
		}
		return fmt.Sprintf("%s: %s to a host this guard cannot read (%s). To allow it, name the host literally and add it to [guards] egress_allow in ~/.fathom/config.toml%s", rule, g.Note, why, off)
	}
	return fmt.Sprintf("%s: %s to %s. To allow it, add %q to [guards] egress_allow in ~/.fathom/config.toml%s", rule, g.Note, t.host, t.host, off)
}

// egressUnreadableWhy is the unknown-target reason for input the lexer
// cannot read.
const egressUnreadableWhy = "unreadable shell input: eval, a function definition, or a command the lexer cannot read"

// egressToolText finds each command check's tool named as a word in raw
// command text, for input the lexer cannot read. A git push and a gh gist
// need their verb too.
var egressToolText = map[string][]*regexp.Regexp{
	"curl_upload":     {egressWordRE("curl")},
	"wget_upload":     {egressWordRE("wget")},
	"socket_tool":     {egressWordRE("nc|ncat|netcat|socat")},
	"remote_copy":     {egressWordRE("scp|rsync|sftp")},
	"ssh_remote":      {egressWordRE("ssh")},
	"git_push_remote": {egressWordRE("git"), egressWordRE("push")},
	"gh_gist_create":  {egressWordRE("gh"), egressWordRE("gist")},
	"cloud_upload":    {egressWordRE("aws|gsutil|gcloud|az")},
}

func egressWordRE(names string) *regexp.Regexp {
	const before, after = "(^|[\\s;&|()'\"`{}!/])", "($|[\\s;&|()'\"`{}])"
	return regexp.MustCompile(before + "(" + names + ")" + after)
}

// egressNamesTool reports whether command's text names check's tool.
func egressNamesTool(check, command string) bool {
	res := egressToolText[check]
	for _, re := range res {
		if !re.MatchString(command) {
			return false
		}
	}
	return len(res) > 0
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
	"--max-redirs", "--noproxy", "--pass", "--preproxy", "--proxy1.0", "--proxy-header", "--quote",
	"--request-target", "--socks4", "--socks4a", "--socks5", "--socks5-hostname", "--speed-limit",
	"--speed-time", "--time-cond", "--url-query", "--mail-from", "--mail-rcpt", "--ciphers",
	"--cert-type", "--key-type", "--ftp-port", "--doh-url", "--local-port", "--expect100-timeout",
	"--keepalive-time", "--hostpubmd5", "--hostpubsha256", "--pinnedpubkey", "--tlsuser",
	"--tlspassword", "--sasl-authzid", "--service-name", "--login-options", "--mail-auth",
	"--engine", "--delegation", "--krb", "--telnet-option",
)}

// curlUpload flags a curl upload by its URL hosts and routes, and any curl
// that reads options the guard cannot see (-K, --config, or a HOME,
// CURL_HOME or XDG_CONFIG_HOME change that loads another .curlrc), since
// those options may add an upload and a URL of their own.
func curlUpload(u *shellcmd.Unit, prior []*shellcmd.Unit) ([]egressTarget, bool) {
	if u.Name() != "curl" {
		return nil, false
	}
	opts, pos := readEgressArgs(u.Words[1:], curlSpec)
	upload := false
	var hidden []egressTarget
	if why := envConfigChange(u, prior, curlConfigVar); why != "" {
		hidden = append(hidden, unknownTarget(why+" can load another .curlrc"))
	}
	for _, o := range opts {
		switch {
		case o.name == "-K" || o.name == "--config":
			hidden = append(hidden, unknownTarget("curl "+o.name+" reads options from a file"))
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
	if !upload && len(hidden) == 0 {
		return nil, false
	}
	targets := wordTargets(pos)
	if len(targets) == 0 {
		targets = []egressTarget{{unknown: true}}
	}
	return append(append(targets, curlRoutes(u, prior, opts)...), hidden...), true
}

// curlConfigVar names a variable that moves curl's .curlrc.
func curlConfigVar(name string) bool {
	return name == "HOME" || name == "CURL_HOME" || name == "XDG_CONFIG_HOME"
}

// proxyVar names a proxy variable (*_proxy or ALL_PROXY, any case, NO_PROXY
// excepted).
func proxyVar(name string) bool {
	name = strings.ToLower(name)
	return name == "all_proxy" || strings.HasSuffix(name, "_proxy") && name != "no_proxy"
}

// envProxies are the proxies a request reaches through the environment: a
// proxy variable prefix is checked like a host, and one an earlier step of
// the command line sets is unknown.
func envProxies(u *shellcmd.Unit, prior []*shellcmd.Unit) []egressTarget {
	var out []egressTarget
	for _, a := range u.Env {
		if proxyVar(a.Name) {
			out = appendProxy(out, a.Value)
		}
	}
	if why := priorEnvChange(prior, proxyVar); why != "" {
		out = append(out, unknownTarget(why+" can set a proxy"))
	}
	return out
}

// curlRoutes are the destinations other than the URL's host that a curl
// request reaches: a proxy (an option or a *_proxy / ALL_PROXY variable,
// any case), or unknown under an option that pins the address or the DNS
// server, so a loopback URL can still leave the machine.
func curlRoutes(u *shellcmd.Unit, prior []*shellcmd.Unit, opts []egressOpt) []egressTarget {
	out := envProxies(u, prior)
	for _, o := range opts {
		switch {
		case o.name == "-x" || o.name == "--proxy" || o.name == "--proxy1.0" || o.name == "--preproxy",
			strings.HasPrefix(o.name, "--socks") && o.has:
			if !o.has {
				out = append(out, egressTarget{unknown: true})
				continue
			}
			out = appendProxy(out, o.value)
		case o.name == "--resolve" || o.name == "--connect-to" || o.name == "--doh-url":
			out = append(out, unknownTarget("curl "+o.name+" routes the request"))
		}
	}
	return out
}

// appendProxy adds a proxy's host; an empty literal proxy is none.
func appendProxy(out []egressTarget, w shellcmd.Word) []egressTarget {
	if w.Literal() && strings.TrimSpace(w.Text) == "" {
		return out
	}
	return append(out, urlTarget(w))
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

// wgetUpload flags a wget upload by its URL hosts and proxies (a proxy
// variable, or an -e/--execute proxy command), and any wget that reads
// commands the guard cannot see (--config, an -e it cannot read, or a
// WGETRC, HOME or XDG_CONFIG_HOME change that loads another wgetrc).
func wgetUpload(u *shellcmd.Unit, prior []*shellcmd.Unit) ([]egressTarget, bool) {
	if u.Name() != "wget" {
		return nil, false
	}
	opts, pos := readEgressArgs(u.Words[1:], wgetSpec)
	upload := false
	routes := envProxies(u, prior)
	var hidden []egressTarget
	if why := envConfigChange(u, prior, wgetConfigVar); why != "" {
		hidden = append(hidden, unknownTarget(why+" can load another wgetrc"))
	}
	for _, o := range opts {
		switch o.name {
		case "--post-data", "--post-file", "--body-data", "--body-file":
			upload = true
		case "--method":
			upload = upload || o.has && o.value.Literal() && writeMethod(o.value.Text)
		case "--config":
			hidden = append(hidden, unknownTarget("wget --config reads commands from a file"))
		case "-e", "--execute":
			if !o.has {
				continue
			}
			if !o.value.Literal() {
				hidden = append(hidden, unknownTarget("a wget "+o.name+" command this guard cannot read"))
				continue
			}
			// wgetrc commands ignore case, "_" and "-".
			key, val, _ := strings.Cut(o.value.Text, "=")
			key = strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(strings.TrimSpace(key)))
			val = strings.TrimSpace(val)
			switch key {
			case "postdata", "postfile", "bodydata", "bodyfile":
				upload = true
			case "method":
				upload = upload || writeMethod(val)
			case "httpproxy", "httpsproxy", "ftpproxy":
				routes = appendProxy(routes, shellcmd.Word{Text: val})
			}
		}
	}
	if !upload && len(hidden) == 0 {
		return nil, false
	}
	targets := wordTargets(pos)
	if upload {
		targets = append(targets, routes...)
	}
	return append(targets, hidden...), true
}

// wgetConfigVar names a variable that moves wget's wgetrc (wget2 reads
// XDG_CONFIG_HOME).
func wgetConfigVar(name string) bool {
	return name == "WGETRC" || name == "HOME" || name == "XDG_CONFIG_HOME"
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
		opts, pos := readEgressArgs(u.Words[1:], spec)
		if len(pos) < 2 {
			return nil, false
		}
		t, remote := remoteSpec(pos[len(pos)-1])
		if !remote {
			return nil, false
		}
		if u.Name() == "rsync" {
			return append([]egressTarget{t}, rsyncShellRoutes(opts)...), true
		}
		return append([]egressTarget{t}, sshRoutes("scp", opts)...), true
	case "sftp":
		opts, pos := readEgressArgs(u.Words[1:], sftpSpec)
		if len(pos) == 0 {
			return nil, false
		}
		w := pos[0]
		t, remote := remoteSpec(w)
		if !remote {
			t = hostTarget(w)
		}
		return append([]egressTarget{t}, sshRoutes("sftp", opts)...), true
	}
	return nil, false
}

// sshRouteLetters are, per tool, the short options that send the
// connection past the named host: a jump host or config file (-J, -F); for
// ssh a forward (-W, -L, -R, -D) or a shared control socket (-S); for scp
// and sftp a program run for the connection (-S).
var sshRouteLetters = map[string]string{"ssh": "JFWLRDS", "scp": "JFS", "sftp": "JFS"}

// sshRoutes is an unknown target when tool's options (ssh, scp, sftp) send
// the connection somewhere other than the named host: a letter of
// sshRouteLetters, or -o ProxyJump|ProxyCommand|HostName, a forward or
// ControlPath (any case, "=" or a space).
func sshRoutes(tool string, opts []egressOpt) []egressTarget {
	for _, o := range opts {
		if len(o.name) == 2 && o.name[0] == '-' && strings.IndexByte(sshRouteLetters[tool], o.name[1]) >= 0 {
			return []egressTarget{unknownTarget(tool + " " + o.name + " routes the connection")}
		}
		switch o.name {
		case "-o":
			if !o.has {
				continue
			}
			if !o.value.Literal() {
				return []egressTarget{unknownTarget("an ssh -o option this guard cannot read")}
			}
			key, _, _ := strings.Cut(strings.TrimSpace(o.value.Text), "=")
			if f := strings.FieldsFunc(key, unicode.IsSpace); len(f) > 0 {
				key = f[0]
			}
			switch k := strings.ToLower(key); k {
			case "proxyjump", "proxycommand", "hostname", "localforward", "remoteforward", "dynamicforward", "controlpath":
				return []egressTarget{unknownTarget("ssh -o " + key + " routes the connection")}
			}
		}
	}
	return nil
}

// rsyncShellRoutes reads rsync's remote shell (-e, --rsh) as an ssh
// command line: its routing options, or unknown when it cannot be read or
// runs a program other than ssh.
func rsyncShellRoutes(opts []egressOpt) []egressTarget {
	for _, o := range opts {
		if o.name != "-e" && o.name != "--rsh" || !o.has {
			continue
		}
		if !o.value.Literal() {
			return []egressTarget{unknownTarget("an rsync remote shell this guard cannot read")}
		}
		c := shellcmd.ParseCommand(o.value.Text)
		units := c.Units()
		if c.Unreadable || len(units) != 1 || len(units[0].Words) == 0 {
			return []egressTarget{unknownTarget("an rsync remote shell this guard cannot read")}
		}
		if units[0].Name() != "ssh" {
			return []egressTarget{unknownTarget("an rsync remote shell other than ssh")}
		}
		ropts, _ := readEgressArgs(units[0].Words[1:], sshSpec)
		return sshRoutes("ssh", ropts)
	}
	return nil
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
	t := hostTarget(w)
	if w.Literal() && strings.Contains(w.Text, "://") {
		t = urlTarget(w)
	}
	return append([]egressTarget{t}, sshRoutes("ssh", opts)...), true
}

// gitInvocation is one git command line read up to its subcommand.
type gitInvocation struct {
	sub      string
	args     []shellcmd.Word // the words after sub
	dir      string          // the -C path, "" for the command's own directory
	dirKnown bool            // false under --git-dir, --work-tree or an unreadable -C
	override bool            // a -c or --config-env config override
}

// readGit reads `git [global options] sub ...`; ok false when u is not git
// or a word before the subcommand is not literal.
func readGit(u *shellcmd.Unit) (gitInvocation, bool) {
	if u.Name() != "git" {
		return gitInvocation{}, false
	}
	words := u.Words[1:]
	inv := gitInvocation{dirKnown: true}
	for i := 0; i < len(words); i++ {
		w := words[i]
		if !w.Literal() {
			return gitInvocation{}, false
		}
		switch t := w.Text; {
		case t == "-C":
			if i+1 >= len(words) || !words[i+1].Literal() {
				inv.dirKnown = false
			} else {
				inv.dir = path.Join(inv.dir, words[i+1].Text)
				if path.IsAbs(words[i+1].Text) {
					inv.dir = words[i+1].Text
				}
			}
			i++
		case t == "-c" || t == "--config-env":
			inv.override = true
			i++
		case strings.HasPrefix(t, "--config-env="):
			inv.override = true
		case t == "--namespace" || t == "--exec-path":
			i++
		case t == "--git-dir" || t == "--work-tree":
			inv.dirKnown = false
			i++
		case strings.HasPrefix(t, "--git-dir=") || strings.HasPrefix(t, "--work-tree="):
			inv.dirKnown = false
		case strings.HasPrefix(t, "-"):
		default:
			inv.sub, inv.args = t, words[i+1:]
			return inv, true
		}
	}
	return inv, true
}

// gitConfigChange names why a push's remote may not be what git reports
// now: a config override on the command, a GIT_*, HOME or XDG_CONFIG_HOME
// variable (a prefix, or set earlier in the command line), or an earlier
// git remote|config|init in the same command line; "" when none.
func gitConfigChange(u *shellcmd.Unit, inv gitInvocation, prior []*shellcmd.Unit) string {
	if inv.override {
		return "a git -c or --config-env override"
	}
	if why := envConfigChange(u, prior, gitConfigVar); why != "" {
		return why
	}
	for _, p := range prior {
		if pi, ok := readGit(p); ok {
			switch pi.sub {
			case "remote", "config", "init":
				return "an earlier git " + pi.sub
			}
		}
	}
	return ""
}

// gitConfigVar names a variable that changes which repository or config
// git reads: GIT_*, and HOME or XDG_CONFIG_HOME (the global gitconfig).
func gitConfigVar(name string) bool {
	return strings.HasPrefix(name, "GIT_") || name == "HOME" || name == "XDG_CONFIG_HOME"
}

// envConfigChange names the variable match accepts that u's own prefix
// sets, or that an earlier step of the command line sets (see
// priorEnvChange); "" when none.
func envConfigChange(u *shellcmd.Unit, prior []*shellcmd.Unit, match func(string) bool) string {
	for _, a := range u.Env {
		if match(a.Name) {
			return "a " + a.Name + " variable"
		}
	}
	return priorEnvChange(prior, match)
}

// priorEnvChange names an earlier step of the command line that sets a
// variable match accepts: a bare assignment (it changes a variable already
// exported), or an export, declare, typeset or readonly naming it. An
// export or declare whose name the guard cannot read counts too; "" when
// none.
func priorEnvChange(prior []*shellcmd.Unit, match func(string) bool) string {
	for _, p := range prior {
		if len(p.Words) == 0 {
			for _, a := range p.Env {
				if match(a.Name) {
					return "an earlier " + a.Name + " assignment"
				}
			}
			continue
		}
		if !p.Words[0].Literal() {
			continue
		}
		switch p.Name() {
		case "export", "declare", "typeset", "readonly":
		default:
			continue
		}
		for _, w := range p.Words[1:] {
			if strings.HasPrefix(w.Text, "-") || strings.HasPrefix(w.Text, "+") {
				continue
			}
			name, _, _ := strings.Cut(w.Text, "=")
			name = strings.TrimSuffix(name, "+")
			if !shellVarName(name) {
				return "an earlier " + p.Name() + " this guard cannot read"
			}
			if match(name) {
				return "an earlier " + p.Name() + " of " + name
			}
		}
	}
	return ""
}

// shellVarName reports a shell variable name: a letter or "_", then
// letters, digits or "_".
func shellVarName(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '_' && !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return s != ""
}

// gitPushDryRunShorts reads a git push short option cluster letter by
// letter: n is a dry run; o takes the rest of the word as its value, or the
// next word when it ends the cluster (next).
func gitPushDryRunShorts(cluster string) (dry, next bool) {
	for k := 0; k < len(cluster); k++ {
		switch cluster[k] {
		case 'n':
			return true, false
		case 'o':
			return false, k+1 == len(cluster)
		}
	}
	return false, false
}

// gitPushRemote reads `git [-C dir] push [options] [repository]`. A
// repository URL is read as written; a remote name, or none, is resolved
// through opts.RemoteURLs. A dry run sends nothing; a config override, a
// GIT_* variable or an earlier git remote|config|init leaves the target
// unknown.
func gitPushRemote(u *shellcmd.Unit, prior []*shellcmd.Unit, opts EgressOptions) ([]egressTarget, bool) {
	inv, ok := readGit(u)
	if !ok || inv.sub != "push" {
		return nil, false
	}
	words := inv.args
	var repo, repoOpt *shellcmd.Word
	for j := 0; j < len(words); j++ {
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
			case "--receive-pack", "--exec", "--push-option":
				if !eq {
					j++
				}
			default:
				if !strings.HasPrefix(t, "--") {
					dry, next := gitPushDryRunShorts(t[1:])
					if dry {
						return nil, false
					}
					if next {
						j++
					}
				}
			}
			continue
		}
		if repo == nil {
			repo = &words[j]
		}
	}
	if why := gitConfigChange(u, inv, prior); why != "" {
		return []egressTarget{unknownTarget(why + " can change where git push sends")}, true
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
		if inv.dirKnown && opts.RemoteURLs != nil {
			urls, ok = opts.RemoteURLs(inv.dir, name)
		}
		if !ok || len(urls) == 0 {
			return []egressTarget{unknownTarget("a git remote this guard cannot resolve")}, true
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
