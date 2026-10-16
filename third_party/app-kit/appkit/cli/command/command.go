// Package command is the cli-kit's cobra+fang bootstrap: a house-default
// root command builder and an Execute that wires the kit's terminal
// palette (appkit/cli/theme) into fang's styled help/errors, then
// translates the result to a process exit code via this package's own
// exit.go convention.
package command

import (
	"context"
	"os"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/theme"
)

// commitAnnotationKey stores App.Commit on the built *cobra.Command's
// Annotations. cobra.Command has no native Commit field (only Version) —
// this is the smallest way to carry it from New to Execute without
// widening Execute's signature past (ctx, root, opts...).
const commitAnnotationKey = "appkit.cli/commit"

// App describes a cli-kit program's identity — the values fang needs for
// styled help, --version, and completions.
type App struct {
	Name    string
	Short   string
	Long    string
	Version string
	Commit  string
}

// New builds the root cobra command with the kit's house defaults:
// SilenceUsage and SilenceErrors are always on — fang.Execute (via
// Execute) owns styled error rendering, and cobra's own unstyled "Error:
// ..." + usage dump on top of that is the bug this silences, not a
// feature. Example, if App carries one via the caller setting it on the
// returned *cobra.Command afterward, passes through untouched — New sets
// no opinion on it.
func New(app App) *cobra.Command {
	root := &cobra.Command{
		Use:           app.Name,
		Short:         app.Short,
		Long:          app.Long,
		Version:       app.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	if app.Commit != "" {
		root.Annotations = map[string]string{commitAnnotationKey: app.Commit}
	}
	return root
}

// Execute runs root through fang — styled help, --version, hidden man
// page, shell completions, ctrl-c handling via WithNotifySignal — with the
// kit's terminal palette (theme.FangColorScheme) wired in by default, and
// translates the result to a process exit code via Code.
//
// opts extend fang's own options (e.g. fang.WithoutManpage); they are
// applied after the house defaults, so a caller can override any of them.
func Execute(ctx context.Context, root *cobra.Command, opts ...fang.Option) int {
	return Code(execute(ctx, root, opts...))
}

// ExecuteWithClassifier preserves explicit Exit codes before allowing classify
// to assign a code to an otherwise unclassified execution error. A nil
// classifier retains Execute's ordinary Code mapping.
func ExecuteWithClassifier(ctx context.Context, root *cobra.Command, classify func(error) (int, bool), opts ...fang.Option) int {
	err := execute(ctx, root, opts...)
	if code, ok := ExplicitCode(err); ok {
		return code
	}
	if err != nil && classify != nil {
		if code, ok := classify(err); ok {
			return code
		}
	}
	return Code(err)
}

func execute(ctx context.Context, root *cobra.Command, opts ...fang.Option) error {
	house := []fang.Option{
		fang.WithColorSchemeFunc(theme.FangColorScheme()),
		fang.WithNotifySignal(os.Interrupt),
	}
	if root.Version != "" {
		house = append(house, fang.WithVersion(root.Version))
	}
	if commit := root.Annotations[commitAnnotationKey]; commit != "" {
		house = append(house, fang.WithCommit(commit))
	}
	house = append(house, opts...)
	return fang.Execute(ctx, root, house...)
}
