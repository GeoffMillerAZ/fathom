// Package benchprep prepares file-based benchmark inputs independently of
// fathom's index, store and model providers.
package benchprep

import (
	"fmt"
	"io/fs"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var identifierToken = regexp.MustCompile(`[\pL_][\pL\pN_]*(?:[./][\pL\pN_][\pL\pN_-]*)*`)

// ExtractIdentifiers specifies the lexical query for BX-8.
// Tokens are Unicode letter/underscore-led words, optionally
// joined by dots or slashes (paths). Outside backticks, only tokens with
// an uppercase letter after the first rune, underscore, dot or slash
// qualify as code-like (a prose word's initial capital is insufficient).
// Inside backticks, all tokens qualify, including plain lowercase names;
// multiword spans contribute separate tokens. CamelCase and snake_case
// stay whole, as do qualified names and paths: no component expansion.
// An unmatched opening backtick quotes the remainder. Tokens shorter than
// three runes and this fixed, case-insensitive stoplist are discarded:
// a an and are as at be been but by can could do does error fail fails false
// file fix for from func has have if in into is issue it nil not of on or
// return should test that the then this to true was when with.
// Results retain spelling, are deduplicated, and sorted by spelling.
func ExtractIdentifiers(issue string) []string {
	const stopwords = " a an and are as at be been but by can could do does error fail fails false file fix for from func has have if in into is issue it nil not of on or return should test that the then this to true was when with "
	seen := map[string]bool{}
	for i, span := range strings.Split(issue, "`") {
		for _, token := range identifierToken.FindAllString(span, -1) {
			if utf8.RuneCountInString(token) < 3 || strings.Contains(stopwords, " "+strings.ToLower(token)+" ") {
				continue
			}
			_, firstWidth := utf8.DecodeRuneInString(token)
			codeLike := strings.ContainsAny(token, "_./") || strings.IndexFunc(token[firstWidth:], unicode.IsUpper) >= 0
			if i%2 == 1 || codeLike {
				seen[token] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for token := range seen {
		out = append(out, token)
	}
	sort.Strings(out)
	return out
}

// LexicalFileRank records a one-based BM25 rank and its evidence.
type LexicalFileRank struct {
	Path  string  `json:"path"`
	Rank  int     `json:"rank"`
	Score float64 `json:"score"`
}

// Stratification keeps both pre-declared definitions for the controller
// to choose later. Each definition is "lexical" or "non-lexical".
type Stratification struct {
	DefinitionA string            `json:"d_a"`
	DefinitionB string            `json:"d_b"`
	LexicalRank int               `json:"lexical_rank"`
	Ranking     []LexicalFileRank `json:"ranking"`
}

// StratifyTask reads a caller-supplied base-tree snapshot, never a live
// index or a patch. All regular UTF-8, NUL-free files participate, including
// tests and docs; .git directories, symlinks and binary files do not.
// Gold paths must name participating files (at least one).
//
// Both documents and query are tokenized with identifierToken and folded
// to lowercase. D-a requires no exact query token in ANY gold file (no
// substring matches). BM25 uses k1=1.2, b=0.75 and positive IDF
// log(1+(N-df+0.5)/(df+0.5)), document lengths in tokens, and unique query
// terms. Terms accumulate in sorted order. All files are ranked, including
// zero-score files; ties use ascending slash-separated path. LexicalRank
// is the best gold rank, so D-b is non-lexical only if ALL gold files rank
// beyond 10. An empty query gives path-order ranks and non-lexical D-a.
func StratifyTask(subject fs.FS, issue string, goldFiles []string) (Stratification, error) {
	if len(goldFiles) == 0 {
		return Stratification{}, fmt.Errorf("stratify: no gold files")
	}
	type document struct {
		path   string
		terms  map[string]int
		length int
	}
	var docs []document
	err := fs.WalkDir(subject, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		raw, err := fs.ReadFile(subject, path)
		if err != nil {
			return err
		}
		if !utf8.Valid(raw) || strings.ContainsRune(string(raw), 0) {
			return nil
		}
		doc := document{path: path, terms: map[string]int{}}
		for _, term := range identifierToken.FindAllString(strings.ToLower(string(raw)), -1) {
			doc.terms[term]++
			doc.length++
		}
		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return Stratification{}, fmt.Errorf("stratify: read base tree: %w", err)
	}
	gold := map[string]bool{}
	for _, path := range goldFiles {
		if !fs.ValidPath(path) {
			return Stratification{}, fmt.Errorf("stratify: invalid gold path %q", path)
		}
		gold[path] = true
	}
	querySet := map[string]bool{}
	for _, term := range ExtractIdentifiers(issue) {
		querySet[strings.ToLower(term)] = true
	}
	query := make([]string, 0, len(querySet))
	for term := range querySet {
		query = append(query, term)
	}
	sort.Strings(query)
	df := map[string]int{}
	totalLength, foundGold := 0, 0
	result := Stratification{DefinitionA: "non-lexical", DefinitionB: "non-lexical"}
	for _, doc := range docs {
		totalLength += doc.length
		if gold[doc.path] {
			foundGold++
		}
		for _, term := range query {
			if doc.terms[term] > 0 {
				df[term]++
				if gold[doc.path] {
					result.DefinitionA = "lexical"
				}
			}
		}
	}
	if foundGold != len(gold) {
		return Stratification{}, fmt.Errorf("stratify: gold file absent from base text corpus")
	}
	averageLength := float64(totalLength) / float64(len(docs))
	for _, doc := range docs {
		score := 0.0
		for _, term := range query {
			tf := float64(doc.terms[term])
			if tf == 0 {
				continue
			}
			idf := math.Log(1 + (float64(len(docs)-df[term])+0.5)/(float64(df[term])+0.5))
			score += idf * tf * 2.2 / (tf + 1.2*(0.25+0.75*float64(doc.length)/averageLength))
		}
		result.Ranking = append(result.Ranking, LexicalFileRank{Path: doc.path, Score: score})
	}
	sort.Slice(result.Ranking, func(i, j int) bool {
		a, b := result.Ranking[i], result.Ranking[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.Path < b.Path
	})
	for i := range result.Ranking {
		result.Ranking[i].Rank = i + 1
		if result.LexicalRank == 0 && gold[result.Ranking[i].Path] {
			result.LexicalRank = i + 1
		}
	}
	if result.LexicalRank <= 10 {
		result.DefinitionB = "lexical"
	}
	return result, nil
}
