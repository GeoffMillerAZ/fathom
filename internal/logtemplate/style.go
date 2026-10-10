package logtemplate

// style.go — a message literal's placeholders, by style. A placeholder is
// read only where the logger (or formatter) renders it: Python's logging,
// pino, winston and SLF4J leave a message's %s or {} as written when the
// call passes no arguments, so a literal "100%" is never a placeholder.

import (
	"strings"
)

// applyStyle splits one decoded literal into parts under style; args
// reports whether the call renders placeholders at all.
func applyStyle(style, s string, args bool, l lang) []part {
	switch style {
	case stylePercent:
		if args {
			return percentParts(s)
		}
	case stylePrintf:
		if args {
			return printfParts(s, l)
		}
	case styleBrace:
		if args {
			return braceParts(s)
		}
	case styleMessageFormat:
		if args {
			return messageFormatParts(s)
		}
	case stylePSR3:
		return psr3Parts(s)
	case "pybrace":
		return pyBraceParts(s)
	}
	return []part{{text: s}}
}

// parts accumulates literal text and holes.
type parts struct {
	out []part
	lit strings.Builder
}

func (p *parts) text(s string) { p.lit.WriteString(s) }
func (p *parts) hole(spell string, isInt bool) {
	p.flush()
	p.out = append(p.out, part{text: spell, hole: true, int: isInt})
}
func (p *parts) flush() {
	if p.lit.Len() > 0 {
		p.out = append(p.out, part{text: p.lit.String()})
		p.lit.Reset()
	}
}
func (p *parts) done() []part {
	p.flush()
	return p.out
}

// percentParts reads Python's %-format: %%, %(name)s, %-5.2f and the like.
func percentParts(s string) []part {
	var p parts
	for i := 0; i < len(s); {
		if s[i] != '%' {
			p.text(s[i : i+1])
			i++
			continue
		}
		if i+1 < len(s) && s[i+1] == '%' {
			p.text("%")
			i += 2
			continue
		}
		j := i + 1
		if j < len(s) && s[j] == '(' {
			k := strings.IndexByte(s[j:], ')')
			if k < 0 {
				p.text("%")
				i++
				continue
			}
			j += k + 1
		}
		for j < len(s) && strings.IndexByte("#0- +", s[j]) >= 0 {
			j++
		}
		j = skipWidth(s, j)
		if j < len(s) && s[j] == '.' {
			j = skipWidth(s, j+1)
		}
		if j < len(s) && strings.IndexByte("hlL", s[j]) >= 0 {
			j++
		}
		if j < len(s) && strings.IndexByte("diouxXeEfFgGcrsa", s[j]) >= 0 {
			p.hole(s[i:j+1], strings.IndexByte("diu", s[j]) >= 0)
			i = j + 1
			continue
		}
		p.text("%")
		i++
	}
	return p.done()
}

func skipWidth(s string, j int) int {
	if j < len(s) && s[j] == '*' {
		return j + 1
	}
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	return j
}

// printfParts reads C-style formats: Java's String.format (%n a newline,
// %1$s indexes), PHP's sprintf (%'.10d padding) and Node's util.format
// (%s %d %i %f %j %o %O %c, no width).
func printfParts(s string, l lang) []part {
	var p parts
	for i := 0; i < len(s); {
		if s[i] != '%' {
			p.text(s[i : i+1])
			i++
			continue
		}
		if i+1 < len(s) && s[i+1] == '%' {
			p.text("%")
			i += 2
			continue
		}
		if l == langJava && i+1 < len(s) && s[i+1] == 'n' {
			p.text("\n")
			i += 2
			continue
		}
		j := i + 1
		if l != langNode {
			k := j
			for k < len(s) && s[k] >= '0' && s[k] <= '9' {
				k++
			}
			if k > j && k < len(s) && s[k] == '$' {
				j = k + 1
			}
			for j < len(s) {
				if strings.IndexByte("-#+ 0,(<", s[j]) >= 0 {
					j++
					continue
				}
				if l == langPHP && s[j] == '\'' && j+1 < len(s) {
					j += 2
					continue
				}
				break
			}
			j = skipWidth(s, j)
			if j < len(s) && s[j] == '.' {
				j = skipWidth(s, j+1)
			}
		}
		if j >= len(s) {
			p.text("%")
			i++
			continue
		}
		c := s[j]
		var conv string
		switch l {
		case langNode:
			conv = "sdifjoOc"
		case langJava:
			conv = "bBhHsScCdoxXeEfgGaAtT"
		default:
			conv = "bcdeEfFgGhHosuxX"
		}
		if strings.IndexByte(conv, c) < 0 {
			p.text("%")
			i++
			continue
		}
		end := j + 1
		if l == langJava && (c == 't' || c == 'T') && end < len(s) {
			end++
		}
		isInt := (l == langNode && c == 'i') || (l != langNode && (c == 'd' || (l == langPHP && c == 'u')))
		p.hole(s[i:end], isInt)
		i = end
	}
	return p.done()
}

// braceParts reads SLF4J's and log4j2's {} anchors: \{} is a literal {},
// \\{} a literal backslash before a placeholder.
func braceParts(s string) []part {
	var p parts
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], `\\{}`):
			p.text(`\`)
			p.hole("{}", false)
			i += 4
		case strings.HasPrefix(s[i:], `\{}`):
			p.text("{}")
			i += 3
		case strings.HasPrefix(s[i:], "{}"):
			p.hole("{}", false)
			i += 2
		default:
			p.text(s[i : i+1])
			i++
		}
	}
	return p.done()
}

// messageFormatParts reads java.text.MessageFormat: {0}, {1,number}; a
// quoted '...' is literal, and two quotes in a row are one.
func messageFormatParts(s string) []part {
	var p parts
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '\'' && i+1 < len(s) && s[i+1] == '\'':
			p.text("'")
			i += 2
		case c == '\'':
			k := strings.IndexByte(s[i+1:], '\'')
			if k < 0 {
				p.text(s[i+1:])
				i = len(s)
				continue
			}
			p.text(s[i+1 : i+1+k])
			i += k + 2
		case c == '{':
			end, depth := -1, 0
			for k := i; k < len(s); k++ {
				if s[k] == '{' {
					depth++
				} else if s[k] == '}' {
					depth--
					if depth == 0 {
						end = k + 1
						break
					}
				}
			}
			if end < 0 || end-i < 3 || s[i+1] < '0' || s[i+1] > '9' {
				p.text("{")
				i++
				continue
			}
			p.hole(s[i:end], strings.Contains(s[i:end], ",number,integer"))
			i = end
		default:
			p.text(s[i : i+1])
			i++
		}
	}
	return p.done()
}

// psr3Parts reads PSR-3 context placeholders: {name}, the name of
// letters, digits, underscore and dot. Monolog's PsrLogMessageProcessor
// renders them; without it the line keeps {name}, which a hole matches
// too.
func psr3Parts(s string) []part {
	var p parts
	for i := 0; i < len(s); {
		if s[i] == '{' {
			j := i + 1
			for j < len(s) && (isIdent(s[j]) && s[j] != '$' || s[j] == '.') {
				j++
			}
			if j > i+1 && j < len(s) && s[j] == '}' {
				p.hole(s[i:j+1], false)
				i = j + 1
				continue
			}
		}
		p.text(s[i : i+1])
		i++
	}
	return p.done()
}

// pyBraceParts reads Python's str.format: {}, {0}, {name:>8}; {{ and }}
// are literal braces.
func pyBraceParts(s string) []part {
	var p parts
	for i := 0; i < len(s); {
		switch {
		case strings.HasPrefix(s[i:], "{{"):
			p.text("{")
			i += 2
		case strings.HasPrefix(s[i:], "}}"):
			p.text("}")
			i += 2
		case s[i] == '{':
			k := strings.IndexByte(s[i:], '}')
			if k < 0 {
				p.text(s[i:])
				i = len(s)
				continue
			}
			p.hole(s[i:i+k+1], false)
			i += k + 1
		default:
			p.text(s[i : i+1])
			i++
		}
	}
	return p.done()
}
