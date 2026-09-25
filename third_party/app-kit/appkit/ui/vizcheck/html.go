package vizcheck

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

func index(root *html.Node) (map[string]*html.Node, error) {
	ids := map[string]*html.Node{}
	count, cells := 0, 0
	var visit func(*html.Node, int) error
	visit = func(n *html.Node, depth int) error {
		count++
		if count > MaxNodes || depth > MaxDepth {
			return fmt.Errorf("vizcheck: HTML exceeds node/depth budget")
		}
		if n.Type == html.ElementNode && n.Namespace == "" && (n.Data == "td" || n.Data == "th") {
			cells++
			if cells > MaxCells {
				return fmt.Errorf("vizcheck: HTML exceeds cell budget")
			}
		}
		seen := map[string]bool{}
		for _, a := range n.Attr {
			key := a.Namespace + ":" + a.Key
			if seen[key] {
				return fmt.Errorf("vizcheck: duplicate HTML attribute")
			}
			seen[key] = true
			if a.Namespace == "" && a.Key == "id" {
				if a.Val == "" || ids[a.Val] != nil {
					return fmt.Errorf("vizcheck: empty or duplicate HTML ID")
				}
				ids[a.Val] = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := visit(c, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return ids, visit(root, 0)
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val
		}
	}
	return ""
}

func has(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			return true
		}
	}
	return false
}

func within(n, scope *html.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if p == scope {
			return true
		}
	}
	return false
}

func walk(n *html.Node, fn func(*html.Node) bool) {
	if !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func elements(n *html.Node, tag string) []*html.Node {
	var found []*html.Node
	namespace := ""
	if tag == "svg" {
		namespace = "svg"
	}
	walk(n, func(v *html.Node) bool {
		if v.Type == html.ElementNode && v.Namespace == namespace && v.Data == tag {
			found = append(found, v)
		}
		return true
	})
	return found
}

func excluded(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	if has(n, "hidden") {
		return true
	}
	switch n.Data {
	case "script", "style", "template", "noscript", "title":
		return true
	}
	if n.Namespace == "svg" {
		switch n.Data {
		case "desc", "defs", "metadata", "symbol", "clipPath", "mask", "pattern", "marker", "filter", "linearGradient", "radialGradient":
			return true
		}
	}
	return false
}

func inertNameReference(n *html.Node) bool {
	// Hidden live text can legitimately supply an ARIA name. Template/script
	// content cannot: unlike hidden elements, it is not a live naming target.
	for p := n; p != nil; p = p.Parent {
		if p.Type == html.ElementNode {
			switch p.Data {
			case "template", "script", "style", "noscript":
				return true
			}
		}
	}
	return false
}

func unavailable(n *html.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if excluded(p) {
			return true
		}
	}
	return false
}

func content(n *html.Node) string {
	var b strings.Builder
	walk(n, func(v *html.Node) bool {
		if excluded(v) {
			return false
		}
		if v.Type == html.TextNode {
			b.WriteString(v.Data)
		}
		return true
	})
	return normalize(b.String())
}

func normalize(s string) string {
	// Collapse only HTML ASCII whitespace, never Unicode spaces or signs.
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
	}), " ")
}
