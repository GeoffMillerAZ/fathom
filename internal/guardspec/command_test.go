package guardspec

import (
	"strings"
	"testing"
)

func TestMatchCommandGuardSegment(t *testing.T) {
	gs := GuardSet{Guards: []Guard{{CommandScopes: []string{"git push*"}, Note: "push"}}}
	for _, tc := range []struct {
		command string
		want    bool
	}{
		{"a && git push --no-verify", true},
		{"x; LEFTHOOK=0 git push", true},
		{"(cd sub && git push -f origin main)", true},
		{"echo ok | git push origin feature", true},
		{"sh -c 'git push --no-verify'", true},
		{"echo $(git push origin feature)", true},
		{"git\tpush origin feature", true},
		{`echo "git push --no-verify"`, false},
		{`echo "x; LEFTHOOK=0 git push"`, false},
		{`git "push --no-verify"`, false},
		{"echo git; echo push", false},
		{"git push 'unterminated", true},
		{"eval 'git push --no-verify'", false},
	} {
		t.Run(tc.command, func(t *testing.T) {
			g, got := MatchCommandGuard(gs, tc.command)
			if got != tc.want || got && g.Note != "push" {
				t.Fatalf("MatchCommandGuard(%q) = %+v, %v; want %v", tc.command, g, got, tc.want)
			}
		})
	}
}

func TestMatchCommandGuardSegmentEnvironmentAndOrder(t *testing.T) {
	gs := GuardSet{Guards: []Guard{
		{CommandScopes: []string{"LEFTHOOK=0 git push*"}, Note: "first"},
		{CommandScopes: []string{"git push*"}, Note: "second"},
	}}
	for _, command := range []string{"x; LEFTHOOK=0 git push", "env LEFTHOOK=0 git push", "git push; LEFTHOOK=0 git push"} {
		g, ok := MatchCommandGuard(gs, command)
		if !ok || g.Note != "first" {
			t.Errorf("%q = %+v, %v; want first guard in declaration order", command, g, ok)
		}
	}
	if _, ok := MatchCommandGuard(GuardSet{Guards: gs.Guards[:1]}, "LEFTHOOK=0 echo ok; git push"); ok {
		t.Fatal("environment from another segment must not leak")
	}
}

func TestGitSafetyNoVerify(t *testing.T) {
	checkGitSafety(t,
		[]string{"git push --no-verify", "a && git push origin feature --no-verify", `git push "--no-verify"`, "git -C sub push --no-verify"},
		[]string{"git push -n --no-verify origin feature", "git push origin feature", `echo "--no-verify"`, `echo "git push --no-verify"`, "git push -n origin feature", `git push -o --no-verify origin feature`, "git push --no-verify --verify origin feature", "git push -- origin --no-verify"},
		"Run git push with hooks enabled")
}

func TestGitSafetyLefthook(t *testing.T) {
	checkGitSafety(t,
		[]string{"LEFTHOOK=0 git push", "x; LEFTHOOK=0 git push", "env LEFTHOOK=0 git push origin feature"},
		[]string{"LEFTHOOK=1 git push", "LEFTHOOK=0 git status", "LEFTHOOK=0 echo ok; git push", "LEFTHOOK=0 LEFTHOOK=1 git push", `echo "LEFTHOOK=0 git push"`},
		"Remove LEFTHOOK=0")
}

func TestGitSafetyForceMain(t *testing.T) {
	checkGitSafety(t,
		[]string{
			"(cd sub && git push -f origin main)", "git push --force origin master",
			"git push --force-with-lease origin main", "git push --force-with-lease=refs/heads/main: origin main",
			"git push origin +main", "git push origin +feature:refs/heads/master",
			"git push -f origin feature:main", "git push -uf origin main", "git push --repo origin -f main",
		},
		[]string{
			"git push -f origin feature", "git push --force-with-lease origin feature", "git push origin +feature",
			"git push -f main feature", "git push -f origin main:feature", "git push origin main",
			"git push -f origin refs/tags/main", "git push -f origin mainish", `echo "git push -f origin main"`,
			"git push origin feature; echo main",
			"git push --force --no-force origin main", "git push --force-with-lease --no-force-with-lease origin main",
			"git push --force-with-lease=refs/heads/feature: origin main",
		},
		"Push main/master without force")
}

func TestGitSafetyGofmtAllowed(t *testing.T) {
	checkGitSafety(t, nil, []string{
		"gofmt -l .", "x && gofmt -l .", "gofmt -l=true .", "gofmt --l .",
		"command gofmt -l .", "gofmt -d .", `echo "gofmt -l ."`, `gofmt -r "x -> -l" .`,
		"gofmt -l -l=false .", "gofmt -- -l", "gofmt file.go -l",
		"gofmt -w file.go", "(gofmt -w .)", "gofmt -w=true .", "command gofmt -w file.go",
		`echo "gofmt -w ."`, "gofmt -w=false .", "gofmt -- -w",
	}, "")
}

func TestGitSafetyLiteralBypasses(t *testing.T) {
	checkGitSafety(t, []string{
		`git push --no-verify origin "$BRANCH"`,
		`git push --no-verify $(git branch --show-current)`,
		`git push -o $option --no-verify`,
		`git --no-pager -P --literal-pathspecs -C dir -c k=v --git-dir=.git push --no-progress --recurse-submodules=check --no-verify`,
		`LEFTHOOK=0 git push origin $B`, `LEFTHOOK=false git push`,
		`LEFTHOOK=NO git push`, `LEFTHOOK=Off git push`,
		`git commit --no-verify`, `git commit -n`, `git commit -anm "message"`, `git commit -o --no-verify`,
	}, []string{`git push -n`, `git commit -m --no-verify`, `git commit -Ssigning-key`, `git commit -unormal`, `git commit --no-verify --verify`, `LEFTHOOK=$value git push`, `git push $flag`}, "")
}

func TestGitSafetyForceExplicitTarget(t *testing.T) {
	checkGitSafety(t, []string{`git push -f`, `git push --force`, `git push -f origin HEAD`, `git push --all -f`}, nil,
		"name the branch: git push --force-with-lease origin <branch>")
}

func TestGitSafetyForceUnknownWarns(t *testing.T) {
	pack, err := GitSafetyPack()
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{`git push -f origin $branch`, `git push -f origin "${BRANCH}"`, `git push origin +$BRANCH`, `git push --force origin $(git branch --show-current)`} {
		g, ok := MatchCommandGuard(pack, command)
		if !ok || g.Enforce || !strings.Contains(g.Note, "unknown") {
			t.Errorf("%q: want unknown warning, got %+v, %v", command, g, ok)
		}
	}
}

func TestMatchCommandGuardFallback(t *testing.T) {
	gs := GuardSet{Guards: []Guard{{CommandScopes: []string{"kubectl delete ns *"}, Note: "keep repo guard", Enforce: true}}}
	for _, command := range []string{`kubectl delete ns $NS`, `kubectl delete ns demo; eval true`, `kubectl delete ns '`} {
		if g, ok := MatchCommandGuard(gs, command); !ok || !g.Enforce {
			t.Errorf("%q: existing whole-string guard must survive, got %+v, %v", command, g, ok)
		}
	}
	for _, tc := range []struct{ pattern, command string }{
		{"sudo *", "sudo rm -rf /tmp/x"},
		{"timeout *", "timeout 30 terraform apply"},
		{"curl *| sh*", "curl https://x.sh | sh"},
		{"* > .env*", "echo SECRET=1 > .env"},
		{"nohup *", "nohup make deploy"},
		{"env FOO=1 *", "env FOO=1 make deploy"},
		{"cd * && make deploy*", "cd sub && make deploy"},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			gs := GuardSet{Guards: []Guard{{CommandScopes: []string{tc.pattern}, Enforce: true}}}
			if g, ok := MatchCommandGuard(gs, tc.command); !ok || !g.Enforce {
				t.Errorf("%q must retain whole-string match for %q, got %+v, %v", tc.command, tc.pattern, g, ok)
			}
		})
	}
}

func TestGitSafetyUnreadableWarns(t *testing.T) {
	pack, err := GitSafetyPack()
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{`git push --no-verify; eval true`, `f(){ :; }; git push --no-verify`, `git push --no-verify '`,
		`sh -c 'git push --no-verify; eval true'`, `(git push --no-verify; eval true)`,
		`echo $(git push --no-verify; eval x)`, "echo `git push --no-verify; eval x`",
		`true&&git push --no-verify; eval true`, `bash -c "git push --no-verify $X"`,
		`eval 'git push --no-verify'`, `sh -c 'git commit -n; eval true'`,
		`xargs git push --no-verify`, `xargs git commit -n`,
	} {
		g, ok := MatchCommandGuard(pack, command)
		if !ok || g.Enforce || !strings.Contains(g.Note, "unknown") {
			t.Errorf("%q: unreadable input must warn, never guess a denial: %+v, %v", command, g, ok)
		}
	}
}

func checkGitSafety(t *testing.T, denied, allowed []string, remedy string) {
	t.Helper()
	pack, err := GitSafetyPack()
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range denied {
		t.Run(command, func(t *testing.T) {
			g, ok := MatchCommandGuard(pack, command)
			if !ok {
				t.Fatal("want matching git-safety guard")
			}
			v := EvaluateCommandEnforcement(g)
			if v == nil || !strings.Contains(v.Remedy, remedy) || strings.ContainsAny(v.Remedy, "\r\n") {
				t.Fatalf("want deny with single remedy line %q, got %+v", remedy, v)
			}
			if v.Cite != "git-safety" {
				t.Errorf("pack citation = %q, want git-safety", v.Cite)
			}
		})
	}
	for _, command := range allowed {
		t.Run(command, func(t *testing.T) {
			if g, ok := MatchCommandGuard(pack, command); ok {
				t.Fatalf("want allow, got %+v", g)
			}
		})
	}
}

func TestGitSafetyRound3Bypasses(t *testing.T) {
	checkGitSafety(t, []string{
		`/usr/bin/git push --no-verify`, `/usr/bin/git commit -n`,
		`git push --no-verif`, `git commit --no-verif`,
		`git push --force-w origin main`, `git push --force-w=main: origin main`,
		`LEFTHOOK=0 git commit`, `export LEFTHOOK=0; git push`,
		`export LEFTHOOK=off; git commit`, `export LEFTHOOK=0 && git push`,
		`sh -c 'export LEFTHOOK=0; git push'`,
		`export LEFTHOOK=0; (git push)`, `export LEFTHOOK=0; echo $(git push)`,
		`git -c core.hooksPath=hooks push`, `git -c core.hooksPath= commit`,
		`git -ccore.hooksPath=hooks push`, `git -c CORE.HOOKSPATH=hooks commit`,
		`git -c core.hooksPath=$HOOKS push`,
	}, []string{
		`git push --no-verif --verif`, `git push --for origin main`,
		`git push --force-w --no-force-w origin main`,
		`git push --force-if-i origin main`, `git commit --no-p`,
		`git -c user.name=Example push`, `git -c core.hooksPath=hooks status`,
		`export LEFTHOOK=0; export LEFTHOOK=1; git push`,
		`export LEFTHOOK=0; LEFTHOOK=1 git commit`,
		`(export LEFTHOOK=0); git push`, `sh -c 'export LEFTHOOK=0'; git push`,
		`echo $(export LEFTHOOK=0); git push`,
		`export LEFTHOOK=$VALUE; git push`, `export LEFTHOOK=0 | cat; git push`,
		`export LEFTHOOK=0; LEFTHOOK=1; git push`,
		`export LEFTHOOK=0; unset LEFTHOOK; git push`,
		`export LEFTHOOK=0; false && export LEFTHOOK=0; git push`,
		`false && export LEFTHOOK=0; git push`,
		`export -n LEFTHOOK=0; git push`,
	}, "")
}

func TestGitSafetyRound3DryRunAndTags(t *testing.T) {
	checkGitSafety(t, []string{`git push --tags -f origin main`, `git push --tags origin +main`}, []string{
		`git push -nf origin main`, `git push -fn origin main`,
		`git push --dry-run -f origin main`, `git push -f origin main --dry-run`,
		`git push -n --no-verify origin main`, `LEFTHOOK=0 git push -n`,
		`git -c core.hooksPath=hooks push --dry-run`,
		`git push --tags -f`, `git push --tags -f origin`,
		`git push --tags --force-with-lease origin`,
	}, "")
}
