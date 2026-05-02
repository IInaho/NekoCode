package tools

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func html2md(rawHTML string) string {
	var b strings.Builder
	z := html.NewTokenizer(strings.NewReader(rawHTML))
	var linkHref string
	skipStack := 0

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}

		if skipStack > 0 {
			if tt == html.EndTagToken {
				name, _ := z.TagName()
				if isSkipAtom(atom.Lookup(name)) {
					skipStack--
				}
			} else if tt == html.StartTagToken {
				name, _ := z.TagName()
				if isSkipAtom(atom.Lookup(name)) {
					skipStack++
				}
			}
			continue
		}

		switch tt {
		case html.TextToken:
			b.WriteString(string(z.Text()))

		case html.StartTagToken:
			name, hasAttr := z.TagName()
			a := atom.Lookup(name)

			var attrs map[string]string
			if hasAttr {
				attrs = getAttrs(z)
			}

			switch a {
			case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
				ensureNewline(&b)
				b.WriteString(strings.Repeat("#", headingLevel(a)) + " ")
			case atom.P, atom.Div, atom.Section, atom.Article:
				ensureNewline(&b)
			case atom.Br:
				b.WriteByte('\n')
			case atom.Li:
				b.WriteString("\n- ")
			case atom.Code:
				b.WriteByte('`')
			case atom.Pre:
				b.WriteString("\n```\n")
			case atom.Strong, atom.B:
				b.WriteString("**")
			case atom.Em, atom.I:
				b.WriteByte('*')
			case atom.A:
				if attrs != nil {
					linkHref = attrs["href"]
				}
				b.WriteByte('[')
			case atom.Img:
				alt := "image"
				src := ""
				if attrs != nil {
					if a, ok := attrs["alt"]; ok && a != "" {
						alt = a
					}
					src = attrs["src"]
				}
				b.WriteString("\n![" + alt + "](" + src + ")\n")
			case atom.Hr:
				b.WriteString("\n---\n")
			case atom.Blockquote:
				b.WriteString("\n> ")
			case atom.Ul, atom.Ol:
				ensureNewline(&b)
			case atom.Script, atom.Style, atom.Svg, atom.Nav, atom.Footer, atom.Header, atom.Aside, atom.Noscript:
				skipStack++
			}

		case html.EndTagToken:
			name, _ := z.TagName()
			a := atom.Lookup(name)
			switch a {
			case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
				b.WriteString("\n\n")
			case atom.P, atom.Div, atom.Section, atom.Article:
				b.WriteString("\n\n")
			case atom.Code:
				b.WriteByte('`')
			case atom.Pre:
				b.WriteString("\n```\n")
			case atom.Strong, atom.B:
				b.WriteString("**")
			case atom.Em, atom.I:
				b.WriteByte('*')
			case atom.A:
				b.WriteString("](" + linkHref + ")")
				linkHref = ""
			case atom.Li:
				// newline already emitted by start tag's "\n- " prefix
			}

		case html.SelfClosingTagToken:
			name, hasAttr := z.TagName()
			a := atom.Lookup(name)

			var attrs map[string]string
			if hasAttr {
				attrs = getAttrs(z)
			}

			switch a {
			case atom.Br:
				b.WriteByte('\n')
			case atom.Hr:
				b.WriteString("\n---\n")
			case atom.Img:
				alt := "image"
				src := ""
				if attrs != nil {
					if a, ok := attrs["alt"]; ok && a != "" {
						alt = a
					}
					src = attrs["src"]
				}
				b.WriteString("\n![" + alt + "](" + src + ")\n")
			}
		}
	}

	return collapseBlankLines(strings.TrimSpace(b.String()))
}

func ensureNewline(b *strings.Builder) {
	s := b.String()
	if len(s) == 0 {
		return
	}
	if s[len(s)-1] != '\n' {
		b.WriteByte('\n')
	}
}

func getAttrs(z *html.Tokenizer) map[string]string {
	m := make(map[string]string)
	for {
		k, v, more := z.TagAttr()
		m[string(k)] = string(v)
		if !more {
			break
		}
	}
	return m
}

func headingLevel(a atom.Atom) int {
	switch a {
	case atom.H1:
		return 1
	case atom.H2:
		return 2
	case atom.H3:
		return 3
	case atom.H4:
		return 4
	case atom.H5:
		return 5
	case atom.H6:
		return 6
	}
	return 1
}

func isSkipAtom(a atom.Atom) bool {
	switch a {
	case atom.Script, atom.Style, atom.Svg, atom.Nav, atom.Footer, atom.Header, atom.Aside, atom.Noscript:
		return true
	}
	return false
}

func collapseBlankLines(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}
