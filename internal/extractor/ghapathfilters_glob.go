package extractor

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type GitHubPathPattern struct {
	Text     string
	re       *regexp.Regexp
	negative bool
}

// Match tests the pattern itself, without the inclusion or exclusion sign.
func (p GitHubPathPattern) Match(path string) bool {
	return p.re.MatchString(path)
}

var (
	errGitHubPathPattern    = errors.New("invalid or unsupported GitHub path pattern")
	gitHubAbsoluteDrivePath = regexp.MustCompile(`^[A-Za-z]:[/\\]`)
)

func compileGitHubPathPattern(text string) (GitHubPathPattern, error) {
	raw, negative := strings.CutPrefix(text, "!")
	if raw == "" || strings.HasPrefix(raw, "/") || gitHubAbsoluteDrivePath.MatchString(raw) || strings.Contains(raw, "${{") || !utf8.ValidString(raw) {
		return GitHubPathPattern{}, errGitHubPathPattern
	}
	var re strings.Builder
	re.WriteString(`\A`)
	quantifiable, segmentStart := false, true
	for len(raw) > 0 {
		r, size := utf8.DecodeRuneInString(raw)
		raw = raw[size:]
		switch r {
		case '*':
			if rest, ok := strings.CutPrefix(raw, "*"); ok {
				raw = rest
				switch {
				case strings.HasPrefix(raw, "*"):
					return GitHubPathPattern{}, errGitHubPathPattern
				case segmentStart && strings.HasPrefix(raw, "/"):
					re.WriteString(`(?:[^/]+/)*`)
					raw = raw[1:]
					continue
				default:
					re.WriteString(`(?s:.*)`)
				}
			} else {
				re.WriteString(`[^/]*`)
			}
			quantifiable = false
		case '+', '?':
			if !quantifiable {
				return GitHubPathPattern{}, errGitHubPathPattern
			}
			re.WriteRune(r)
			quantifiable = false
		case '[':
			class, rest, ok := strings.Cut(raw, "]")
			if !ok || !validGitHubPathClass(class) {
				return GitHubPathPattern{}, errGitHubPathPattern
			}
			re.WriteString("[" + class + "]")
			raw, quantifiable = rest, true
		case '\\':
			escaped, n := utf8.DecodeRuneInString(raw)
			if n == 0 || !strings.ContainsRune(`*?+[]!\{}`, escaped) {
				return GitHubPathPattern{}, errGitHubPathPattern
			}
			re.WriteString(regexp.QuoteMeta(string(escaped)))
			raw, quantifiable = raw[n:], true
		case ']', '{', '}':
			return GitHubPathPattern{}, errGitHubPathPattern
		default:
			if unicode.IsControl(r) {
				return GitHubPathPattern{}, errGitHubPathPattern
			}
			re.WriteString(regexp.QuoteMeta(string(r)))
			quantifiable = r != '/'
		}
		segmentStart = r == '/'
	}
	re.WriteString(`\z`)
	compiled, err := regexp.Compile(re.String())
	if err != nil {
		return GitHubPathPattern{}, errGitHubPathPattern
	}
	return GitHubPathPattern{Text: text, re: compiled, negative: negative}, nil
}

func validGitHubPathClass(class string) bool {
	if class == "" {
		return false
	}
	for i := 0; i < len(class); i++ {
		group := gitHubPathClassGroup(class[i])
		if group == 0 {
			return false
		}
		if i+1 < len(class) && class[i+1] == '-' {
			if i+2 >= len(class) || gitHubPathClassGroup(class[i+2]) != group || class[i] > class[i+2] {
				return false
			}
			i += 2
		}
	}
	return true
}

func gitHubPathClassGroup(b byte) byte {
	switch {
	case '0' <= b && b <= '9':
		return '0'
	case 'A' <= b && b <= 'Z':
		return 'A'
	case 'a' <= b && b <= 'z':
		return 'a'
	default:
		return 0
	}
}
