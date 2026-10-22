package semantic

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The initial-verb list is deliberately closed: expanding it changes which
// uncompiled sections can consume an obligation slot.
const rawInitialVerbs = "Use Put Keep Run Add Write Read Prefer Avoid Check Extend Sort Inject Pass Return Record Treat Make Call Wrap Start Stop Ask Name Mark Route"

// Additional unambiguous base forms after sentence-initial Only. This list is
// separate so the fixed bare-imperative list above does not silently expand.
const rawOnlyVerbs = "Serve Emit Select Cite Include Compile Modify Remove Delete Load Save Build Fetch Send Show Count Match Parse Filter Verify Validate Apply Update Create Set Limit Allow Deny Reject Accept Retain Drop Skip Reuse Explain Display Report Enforce Resolve Refresh Rank Extract Store Preserve"

var (
	rawADRHeading     = regexp.MustCompile(`^#+\s+ADR-[0-9]+\b`)
	rawTableSeparator = regexp.MustCompile(`^\|?\s*:?-{3,}:?\s*(?:\|\s*:?-{3,}:?\s*)+\|?$`)
	rawLabel          = regexp.MustCompile(`^\*\*([A-Z][^*]*)\*\*:?\s*`)
	rawList           = regexp.MustCompile(`^\s*(?:[-+*]|[0-9]+\.)\s+`)
	rawNumbered       = regexp.MustCompile(`^\s*[0-9]+\.\s+`)
	rawLink           = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	rawModal          = regexp.MustCompile(`(?i)\b(?:must|never|always|shall|do not|don['’]t|required to)\b`)
	rawOnly           = regexp.MustCompile(`(?i)\b(?:may only|must only)\b`)
)

// rawRuleText extracts at most two deterministic, short instructions from a
// raw section. An empty result means there is no safe rule to serve.
func rawRuleText(sec Section) string {
	lines := rawVisibleLines(sec.Text)
	if len(lines) == 0 {
		return ""
	}
	if rawADRHeading.MatchString(lines[0]) {
		return rawADRDecisionText(lines)
	}
	if sec.Path == "ADR.md" {
		return ""
	}
	blocks := rawBlocks(lines, sec.Path)
	var out []string
	for _, block := range blocks {
		for _, sentence := range rawSentences(rawPlain(block)) {
			if !rawImperative(sentence) {
				continue
			}
			out = append(out, rawCap(sentence))
			if len(out) == 2 {
				return strings.Join(out, "\n")
			}
		}
	}
	return strings.Join(out, "\n")
}

func rawVisibleLines(text string) []string {
	var out []string
	fence := ""
	comment := false
	inTable := false
	for _, original := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(original)
		if fence != "" {
			if strings.HasPrefix(line, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(line, "```") {
			fence = "```"
			continue
		}
		if strings.HasPrefix(line, "~~~") {
			fence = "~~~"
			continue
		}
		for {
			if comment {
				end := strings.Index(line, "-->")
				if end < 0 {
					line = ""
					break
				}
				line = line[end+3:]
				comment = false
			}
			start := rawOutsideCodeIndex(line, "<!--")
			if start < 0 {
				break
			}
			end := strings.Index(line[start+4:], "-->")
			if end >= 0 {
				line = line[:start] + line[start+4+end+3:]
				continue
			}
			line = line[:start]
			comment = true
			break
		}
		line = strings.TrimSpace(line)
		if rawTableSeparator.MatchString(line) {
			if len(out) > 0 {
				out[len(out)-1] = ""
			}
			inTable = true
			out = append(out, "")
			continue
		}
		if inTable {
			if rawOutsideCodeIndex(line, "|") >= 0 {
				out = append(out, "")
				continue
			}
			inTable = false
		}
		if rawTableLine(line) {
			out = append(out, "")
			continue
		}
		out = append(out, line)
	}
	return out
}

func rawBlocks(lines []string, path string) []string {
	var blocks []string
	current := ""
	skipChecked := false
	flush := func() {
		if current != "" {
			blocks = append(blocks, current)
			current = ""
		}
	}
	for _, line := range lines {
		if line == "" {
			flush()
			continue
		}
		if strings.HasPrefix(line, "#") {
			flush()
			skipChecked = false
			continue
		}
		if rawList.MatchString(line) {
			flush()
			current = rawList.ReplaceAllString(line, "")
			skipChecked = strings.HasPrefix(path, "docs/plans/") && (strings.HasPrefix(current, "[x] ") || strings.HasPrefix(current, "[X] "))
			if skipChecked {
				current = ""
				continue
			}
			if strings.HasPrefix(path, "docs/plans/") {
				current = strings.TrimPrefix(current, "[ ] ")
			}
			continue
		}
		if skipChecked {
			continue
		}
		if current == "" {
			current = line
		} else {
			current = rawJoin(current, line)
		}
	}
	flush()
	return blocks
}

func rawADRDecisionText(lines []string) string {
	inside := false
	type item struct {
		text     string
		numbered bool
	}
	var items []item
	current := item{}
	flush := func() {
		if current.text != "" {
			items = append(items, current)
			current = item{}
		}
	}
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "#") {
			break
		}
		if label := rawLabel.FindStringSubmatch(line); label != nil {
			name := strings.ToLower(strings.TrimRight(strings.TrimSpace(label[1]), ".:"))
			if !inside {
				if name == "decision" || name == "decisions" {
					inside = true
					line = strings.TrimSpace(strings.TrimPrefix(line, label[0]))
				} else {
					continue
				}
			} else {
				break
			}
		}
		if !inside {
			continue
		}
		if line == "" {
			flush()
			continue
		}
		if rawNumbered.MatchString(line) {
			flush()
			current = item{text: rawNumbered.ReplaceAllString(line, ""), numbered: true}
			continue
		}
		if current.text == "" {
			current.text = line
		} else {
			current.text = rawJoin(current.text, line)
		}
	}
	flush()
	if !inside {
		return ""
	}
	var out []string
	for _, block := range items {
		plain := rawPlain(block.text)
		sentences := rawSentences(plain)
		if len(sentences) > 0 && !rawCompleteSentence(sentences[0]) {
			continue
		}
		if block.numbered {
			if len(sentences) > 0 {
				out = append(out, rawCap(sentences[0]))
			}
		} else if len(sentences) > 0 {
			// A decision paragraph supplies its first two sentences; numbered
			// items supply one sentence apiece.
			for _, sentence := range sentences {
				if !rawCompleteSentence(sentence) {
					break
				}
				out = append(out, rawCap(sentence))
				if len(out) == 2 {
					return strings.Join(out, "\n")
				}
			}
		}
		if len(out) == 2 {
			break
		}
	}
	return strings.Join(out, "\n")
}

func rawCompleteSentence(s string) bool {
	return strings.HasSuffix(s, ".") || strings.HasSuffix(s, "!") || strings.HasSuffix(s, "?")
}

func rawPlain(s string) string {
	s = rawStripLinks(s)
	var out strings.Builder
	for len(s) > 0 {
		i := strings.IndexByte(s, '`')
		if i < 0 {
			out.WriteString(rawPlainOutside(s))
			break
		}
		out.WriteString(rawPlainOutside(s[:i]))
		run := 1
		for i+run < len(s) && s[i+run] == '`' {
			run++
		}
		end := strings.Index(s[i+run:], strings.Repeat("`", run))
		if end < 0 {
			out.WriteString(s[i:])
			break
		}
		end += i + run + run
		out.WriteString(s[i:end])
		s = s[end:]
	}
	return strings.TrimSpace(out.String())
}

func rawPlainOutside(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '*' || (s[i] == '_' && rawBoundaryBefore(s, i)) {
			mark := string(s[i])
			if i+1 < len(s) && s[i+1] == s[i] {
				mark += mark
			}
			if i+len(mark) < len(s) && !unicode.IsSpace(rune(s[i+len(mark)])) {
				if end := strings.Index(s[i+len(mark):], mark); end > 0 {
					end += i + len(mark)
					if !unicode.IsSpace(rune(s[end-1])) && (s[i] == '*' || rawBoundaryAfter(s, end+len(mark))) {
						out.WriteString(s[i+len(mark) : end])
						i = end + len(mark)
						continue
					}
				}
			}
		}
		out.WriteByte(s[i])
		i++
	}
	core := strings.Join(strings.Fields(out.String()), " ")
	if core == "" {
		return rawTrailingSpace(s)
	}
	if unicode.IsSpace(rune(s[0])) {
		core = " " + core
	}
	return core + rawTrailingSpace(s)
}

func rawStripLinks(s string) string {
	matches := rawLink.FindAllStringSubmatchIndex(s, -1)
	if len(matches) == 0 {
		return s
	}
	var out strings.Builder
	last := 0
	for _, match := range matches {
		if rawCodeAt(s, match[0]) {
			continue
		}
		out.WriteString(s[last:match[0]])
		out.WriteString(s[match[2]:match[3]])
		last = match[1]
	}
	out.WriteString(s[last:])
	return out.String()
}

func rawOutsideCodeIndex(s, needle string) int {
	for at := 0; at < len(s); {
		if s[at] == '`' {
			run := 1
			for at+run < len(s) && s[at+run] == '`' {
				run++
			}
			if end := strings.Index(s[at+run:], strings.Repeat("`", run)); end >= 0 {
				at += run + end + run
				continue
			}
			return -1
		}
		if strings.HasPrefix(s[at:], needle) {
			return at
		}
		at++
	}
	return -1
}

func rawCodeAt(s string, target int) bool {
	for at := 0; at < target; {
		if s[at] != '`' {
			at++
			continue
		}
		run := 1
		for at+run < len(s) && s[at+run] == '`' {
			run++
		}
		if end := strings.Index(s[at+run:], strings.Repeat("`", run)); end >= 0 {
			end += at + run + run
			if target < end {
				return true
			}
			at = end
			continue
		}
		return true
	}
	return false
}

func rawBoundaryBefore(s string, i int) bool {
	return i == 0 || unicode.IsSpace(rune(s[i-1])) || strings.ContainsRune("([{\"'", rune(s[i-1]))
}

func rawBoundaryAfter(s string, i int) bool {
	return i == len(s) || unicode.IsSpace(rune(s[i])) || strings.ContainsRune(".,:;!?)]}\"'", rune(s[i]))
}

func rawTrailingSpace(s string) string {
	if len(s) > 0 && unicode.IsSpace(rune(s[len(s)-1])) {
		return " "
	}
	return ""
}

func rawJoin(left, right string) string {
	if strings.HasSuffix(left, "/") {
		return left + right
	}
	return left + " " + right
}

func rawTableLine(line string) bool {
	return strings.HasPrefix(line, "|") || strings.HasSuffix(line, "|") || rawOutsideCodeIndex(line, " | ") >= 0
}

func rawSentences(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '`' {
			run := 1
			for i+run < len(s) && s[i+run] == '`' {
				run++
			}
			if end := strings.Index(s[i+run:], strings.Repeat("`", run)); end >= 0 {
				i += run + end + run - 1
				continue
			}
		}
		if s[i] != '.' && s[i] != '!' && s[i] != '?' {
			continue
		}
		if i+1 < len(s) && s[i+1] != ' ' {
			continue
		}
		if s[i] == '.' && rawNonterminalDot(s, i) {
			continue
		}
		part := strings.TrimSpace(s[start : i+1])
		if part != "" {
			out = append(out, part)
		}
		start = i + 1
	}
	if tail := strings.TrimSpace(s[start:]); tail != "" {
		out = append(out, tail)
	}
	return out
}

func rawNonterminalDot(s string, i int) bool {
	if i > 0 && i+1 < len(s) && s[i-1] >= '0' && s[i-1] <= '9' && s[i+1] >= '0' && s[i+1] <= '9' {
		return true
	}
	if i > 0 && s[i-1] >= '0' && s[i-1] <= '9' && (i+1 == len(s) || s[i+1] == ' ') {
		j := i - 1
		for j >= 0 && s[j] >= '0' && s[j] <= '9' {
			j--
		}
		if j < 0 || s[j] == ' ' {
			return true
		}
	}
	lo := i - 1
	for lo >= 0 && (unicode.IsLetter(rune(s[lo])) || s[lo] == '.') {
		lo--
	}
	word := strings.ToLower(s[lo+1 : i+1])
	return word == "e.g." || word == "i.e." || word == "vs." || word == "e." || word == "i."
}

func rawImperative(s string) bool {
	if rawModal.MatchString(s) || rawOnly.MatchString(s) {
		return true
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return false
	}
	first := strings.Trim(words[0], "*`_.,:;!?")
	if strings.EqualFold(first, "Only") && len(words) > 1 {
		return rawInitialVerb(words[1]) || rawOnlyVerb(words[1])
	}
	return rawInitialVerb(first)
}

func rawOnlyVerb(s string) bool {
	s = strings.Trim(s, "*`_.,:;!?")
	for _, verb := range strings.Fields(rawOnlyVerbs) {
		if strings.EqualFold(s, verb) {
			return true
		}
	}
	return false
}

func rawInitialVerb(s string) bool {
	s = strings.Trim(s, "*`_.,:;!?")
	for _, verb := range strings.Fields(rawInitialVerbs) {
		if strings.EqualFold(s, verb) {
			return true
		}
	}
	return false
}

func rawCap(s string) string {
	if utf8.RuneCountInString(s) <= 240 {
		return s
	}
	runes := []rune(s)
	cut := 239 // reserve the final rune for ellipsis
	for cut > 0 && !unicode.IsSpace(runes[cut]) {
		cut--
	}
	if cut == 0 {
		return "…"
	}
	return strings.TrimSpace(string(runes[:cut])) + "…"
}
