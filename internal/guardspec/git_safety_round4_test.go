package guardspec

import (
	"strings"
	"testing"
)

func TestGitSafetyRemoteDeletionAndMirror(t *testing.T) {
	checkGitSafety(t, []string{
		"git push origin :main", "git push origin :refs/heads/master",
		"git push -d origin main", "git push --delete origin master",
		"git push --delete origin feature main", "git push --del origin main",
		"git push --mirror", "git push --mirror origin", "git push --mirror --tags",
		"git -c alias.x=y push --force origin main", "git --config-env=core.hooksPath=H push --no-verify",
	}, []string{
		"git push origin :feature", "git push -d origin feature", "git push --delete origin refs/tags/main",
		"git push --delete --no-delete origin main", "git push --mirror --no-mirror origin main",
		"git push --delete origin main --dry-run", "git push --mirror -n",
		"git reset --hard", "git branch -D main", "git tag -d main", "git clean -fd", "git checkout -- .",
	}, "")
}

func TestGitSafetyConfigIndirectionWarns(t *testing.T) {
	pack, err := GitSafetyPack()
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{
		`git -c alias.publish=push publish`, `git -calias.publish=push publish --no-verify`,
		`git -c alias.publish=push push origin feature`, `git -c ALIAS.publish=push publish`,
		`git --config-env=core.hooksPath=HOOKS push origin feature`,
		`git --config-env core.hooksPath=HOOKS commit -m x`,
		`git --config-env=alias.publish=PUBLISH publish`,
	} {
		t.Run(command, func(t *testing.T) {
			g, ok := MatchCommandGuard(pack, command)
			if !ok || g.Enforce || !strings.Contains(g.Note, "unknown") {
				t.Fatalf("configuration indirection must warn, never deny: %+v, %v", g, ok)
			}
		})
	}
}

func TestGitSafetyRuleIDs(t *testing.T) {
	pack, err := GitSafetyPack()
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range pack.Guards {
		v := EvaluateCommandEnforcement(g)
		if v == nil || v.RuleID != "git-safety/"+g.commandCheck {
			t.Errorf("pack identity must name its check: %+v", v)
		}
	}
}
