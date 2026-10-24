package guardspec

import (
	_ "embed"
	"fmt"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"

	"github.com/geoffmilleraz/fathom/internal/shellcmd"
)

//go:embed git_safety.cue
var gitSafetyData []byte

// GitSafetyPack loads the shipped default safety guards from embedded data.
// Callers match this set with MatchCommandGuard and pass a matched guard to
// EvaluateCommandEnforcement. Loading neither reads configuration nor activates
// hooks. Predicates are pack-private; callers apply the guards.git_safety opt-out.
func GitSafetyPack() (GuardSet, error) {
	v := cuecontext.New().CompileBytes(gitSafetyData, cue.Filename("git-safety"))
	if err := v.Err(); err != nil {
		return GuardSet{}, err
	}
	gs, err := decodeGuards("git-safety", v)
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
		switch g.Check {
		case "git_no_verify", "push_lefthook_off", "git_hooks_path", "push_force_protected":
			gs.Guards[i].commandCheck = g.Check
		default:
			return GuardSet{}, fmt.Errorf("git-safety: unknown command check %q", g.Check)
		}
	}
	return gs, nil
}

// matchSafetyGuard warns on alias or --config-env indirection unless a literal
// check denies: an alias cannot shadow the push or commit builtins.
func matchSafetyGuard(g Guard, u *shellcmd.Unit) (Guard, bool) {
	matched, ok := matchSafetyLiteral(g, u)
	if p, _ := parseSafetyGit(u); p.configUnknown && !(ok && matched.Enforce) {
		g.Enforce = false
		g.Note = "git-safety: command safety is unknown (git aliases or environment configuration); run git with hooks enabled and name the target branch."
		return g, true
	}
	return matched, ok
}

// matchSafetyLiteral never expands words or infers repository configuration.
func matchSafetyLiteral(g Guard, u *shellcmd.Unit) (Guard, bool) {
	if u.Name() == "xargs" && safetyGitText.MatchString(strings.Join(u.Args()[1:], " ")) && safetyActionText.MatchString(strings.Join(u.Args()[1:], " ")) {
		g.Enforce = false
		g.Note = "git-safety: command safety is unknown (xargs constructs git arguments); run git with hooks enabled and name the target branch."
		return g, true
	}
	p, ok := parseSafetyGit(u)
	if !ok || p.subcommand == "push" && p.dryRun {
		return Guard{}, false
	}
	switch g.commandCheck {
	case "git_no_verify":
		return g, p.noVerify
	case "git_hooks_path":
		return g, p.hooksPath
	case "push_lefthook_off":
		value := ""
		for _, a := range u.Env {
			if a.Name == "LEFTHOOK" {
				value = ""
				if !a.Append && a.Value.Literal() {
					value = strings.ToLower(a.Value.Text)
				}
			}
		}
		return g, value == "0" || value == "false" || value == "no" || value == "off"
	case "push_force_protected":
		if p.subcommand != "push" {
			return Guard{}, false
		}
		force := p.force || p.lease || len(p.leaseRefs) > 0 || p.delete || p.mirror
		unknown, implicit := p.unknown && force, p.mirror || force && len(p.refs) == 0 && !p.tags
		for _, word := range p.refs {
			if !word.Literal() {
				unknown = unknown || force || strings.HasPrefix(word.Text, "+")
				continue
			}
			ref := word.Text
			forced := p.force || p.lease || p.delete || strings.HasPrefix(ref, "+")
			ref = strings.TrimPrefix(ref, "+")
			if src, dst, found := strings.Cut(ref, ":"); found {
				forced = forced || src == "" // an empty source deletes the remote ref
				ref = dst
			}
			for _, leaseRef := range p.leaseRefs {
				forced = forced || leaseRef == ref || leaseRef == "refs/heads/"+ref || ref == "refs/heads/"+leaseRef
			}
			if forced && (ref == "main" || ref == "master" || ref == "refs/heads/main" || ref == "refs/heads/master") {
				return g, true
			}
			implicit = implicit || forced && (ref == "HEAD" || ref == "")
		}
		if implicit && !p.unknown {
			g.Note = "name the branch: git push --force-with-lease origin <branch>"
			return g, true
		}
		if unknown {
			g.Enforce = false
			g.Note = "git-safety: force-push target is unknown; name the branch: git push --force-with-lease origin <branch>"
			return g, true
		}
	}
	return Guard{}, false
}

type safetyPush struct {
	subcommand                      string
	noVerify, force, lease, unknown bool
	dryRun, tags, hooksPath         bool
	delete, mirror, configUnknown   bool
	refs                            []shellcmd.Word
	leaseRefs                       []string
}

// parseSafetyGit reads literal flags even when other words need expansion.
// Unknown options do not hide later flags. Only target analysis is undecidable.
func parseSafetyGit(u *shellcmd.Unit) (safetyPush, bool) {
	var out safetyPush
	args := u.Words
	if len(args) == 0 || !args[0].Literal() || u.Name() != "git" {
		return out, false
	}
	i := 1
	for i < len(args) && args[i].Literal() && strings.HasPrefix(args[i].Text, "-") {
		arg := args[i].Text
		switch arg {
		case "-c":
			if i+1 < len(args) {
				out.hooksPath = out.hooksPath || safetyHooksPath(args[i+1])
				out.configUnknown = out.configUnknown || strings.HasPrefix(strings.ToLower(args[i+1].Text), "alias.")
			}
			i += 2
		case "--config-env":
			out.configUnknown = true
			i += 2
		case "-C", "--git-dir", "--work-tree", "--namespace":
			i += 2
		default:
			if strings.HasPrefix(arg, "-c") && !strings.HasPrefix(arg, "--") {
				out.hooksPath = out.hooksPath || safetyHooksPath(shellcmd.Word{Text: arg[2:]})
				out.configUnknown = out.configUnknown || strings.HasPrefix(strings.ToLower(arg[2:]), "alias.")
			}
			out.configUnknown = out.configUnknown || strings.HasPrefix(arg, "--config-env=")
			i++
		}
	}
	if i >= len(args) || !args[i].Literal() || (args[i].Text != "push" && args[i].Text != "commit") {
		return out, false
	}
	out.subcommand = args[i].Text
	i++
	repository, options := false, true
	for ; i < len(args); i++ {
		word := args[i]
		if !word.Literal() {
			out.unknown = true
			if repository {
				out.refs = append(out.refs, word)
			} else {
				repository = true
			}
			continue
		}
		arg := word.Text
		if options && arg == "--" {
			options = false
			continue
		}
		if options && strings.HasPrefix(arg, "-") {
			key, value, hasValue := strings.Cut(arg, "=")
			key = safetyGitLongOption(out.subcommand, key)
			switch key {
			case "--delete":
				if !hasValue {
					out.delete = true
				}
			case "--no-delete":
				if !hasValue {
					out.delete = false
				}
			case "--mirror":
				if !hasValue {
					out.mirror = true
				}
			case "--no-mirror":
				if !hasValue {
					out.mirror = false
				}
			case "--dry-run":
				if !hasValue && out.subcommand == "push" {
					out.dryRun = true
				}
			case "--tags":
				if !hasValue {
					out.tags = true
				}
			case "--no-tags":
				if !hasValue {
					out.tags = false
				}
			case "--no-verify":
				if !hasValue {
					out.noVerify = true
				}
			case "--verify":
				if !hasValue {
					out.noVerify = false
				}
			case "--force":
				if !hasValue {
					out.force = true
				}
			case "--no-force":
				if !hasValue {
					out.force = false
				}
			case "--force-with-lease":
				if !hasValue {
					out.lease = true
				} else {
					ref, _, _ := strings.Cut(value, ":")
					out.leaseRefs = append(out.leaseRefs, ref)
				}
			case "--no-force-with-lease":
				if !hasValue {
					out.lease = false
					out.leaseRefs = nil
				}
			case "--repo", "--receive-pack", "--exec", "--push-option", "-o":
				if key == "-o" && out.subcommand == "commit" {
					continue
				}
				if !hasValue {
					i++
				}
				if key == "--repo" {
					repository = true
				}
			default:
				if strings.HasPrefix(arg, "--") {
					if out.subcommand == "commit" && !hasValue && (key == "--message" || key == "--file" || key == "--author" || key == "--date" || key == "--template" || key == "--cleanup" || key == "--reuse-message" || key == "--reedit-message" || key == "--fixup" || key == "--squash" || key == "--trailer" || key == "--pathspec-from-file") {
						i++
					}
					continue
				}
				for j, flag := range arg[1:] {
					if flag == 'n' {
						if out.subcommand == "commit" {
							out.noVerify = true
						} else {
							out.dryRun = true
						}
					}
					if flag == 'f' && out.subcommand == "push" {
						out.force = true
					}
					if flag == 'd' && out.subcommand == "push" {
						out.delete = true
					}
					if out.subcommand == "commit" && (flag == 'S' || flag == 'u') {
						break
					}
					if flag == 'o' && out.subcommand == "push" || out.subcommand == "commit" && strings.ContainsRune("mFCRct", flag) {
						if j == len(arg)-2 {
							i++
						}
						break
					}
				}
			}
		} else if !repository {
			repository = true
		} else {
			out.refs = append(out.refs, word)
		}
	}
	return out, true
}

// The key must be literal even when the configured value needs expansion.
func safetyHooksPath(word shellcmd.Word) bool {
	key, _, assigned := strings.Cut(word.Text, "=")
	return assigned && strings.EqualFold(key, "core.hooksPath")
}

// Git accepts exact long options or an unambiguous prefix, including negated
// forms. Include the other options too: --for is ambiguous and cannot prove
// --force. These are the push/commit parse-options names, not runtime config.
func safetyGitLongOption(subcommand, key string) string {
	if !strings.HasPrefix(key, "--") {
		return key
	}
	options := "quiet verbose all dry-run porcelain verify"
	if subcommand == "push" {
		options += " repo branches mirror delete tags force force-with-lease force-if-includes recurse-submodules thin receive-pack exec set-upstream progress prune follow-tags signed atomic push-option ipv4 ipv6"
	} else {
		options += " file author date message reedit-message reuse-message fixup squash reset-author trailer signoff template edit cleanup status gpg-sign include interactive patch unified inter-hunk-context only short branch ahead-behind long null amend post-rewrite untracked-files pathspec-from-file pathspec-file-nul"
	}
	match := ""
	for _, option := range strings.Fields(options) {
		for _, name := range []string{"--" + option, "--no-" + option} {
			if key == name {
				return key
			}
			if strings.HasPrefix(name, key) {
				if match != "" && match != name {
					return key // ambiguous; never guess a denial
				}
				match = name
			}
		}
	}
	if match != "" {
		return match
	}
	return key
}
