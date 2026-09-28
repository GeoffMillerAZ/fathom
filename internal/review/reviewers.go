package review

import "sort"

// Owner-suggestion sources (design doc §2): CODEOWNERS rules for the
// changed and impacted files, then the most frequent committers to the
// impacted providers in 365 days — both already resolved by the caller
// (internal/ownership does the CODEOWNERS parse and the git-log window;
// this leaf package never reads a file or runs git, same discipline as
// every other Input field).
const (
	OwnerSourceCodeowners        = "codeowners"
	OwnerSourceFrequentCommitter = "frequent_committer"
)

// OwnerFiles is one suggested reviewer, with the files that put them there
// — deliberately NO rank, score or commit count (I58: "no instrument, view
// or export ranks, scores or compares human beings"). Owner is an identity
// string (a CODEOWNERS handle or a committer identity) the caller already
// resolved; this package never looks one up.
type OwnerFiles struct {
	Owner  string
	Files  []string
	Source string
}

// suggestReviewers merges CODEOWNERS matches and frequent-committer matches
// into one review-suggestion list (CX-4). The only ordering applied is
// alphabetical by owner then source — NEVER by commit count, frequency or
// any other productivity measure (I58; design doc §2: "never ranked by
// productivity"). Files are deduped and sorted within each entry.
func suggestReviewers(codeowners, frequentCommitters []OwnerFiles) []OwnerFiles {
	out := make([]OwnerFiles, 0, len(codeowners)+len(frequentCommitters))
	for _, o := range codeowners {
		out = append(out, normalizeOwnerFiles(o, OwnerSourceCodeowners))
	}
	for _, o := range frequentCommitters {
		out = append(out, normalizeOwnerFiles(o, OwnerSourceFrequentCommitter))
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Owner != out[j].Owner {
			return out[i].Owner < out[j].Owner
		}
		return out[i].Source < out[j].Source
	})
	return out
}

func normalizeOwnerFiles(o OwnerFiles, source string) OwnerFiles {
	files := dedupeSorted(o.Files)
	return OwnerFiles{Owner: o.Owner, Files: files, Source: source}
}

func dedupeSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
