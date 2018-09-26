package tg_md2html

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

var openHTML = map[rune][]rune{
	'_': []rune("<i>"),
	'*': []rune("<b>"),
	'`': []rune("<code>"),
}

var closeHTML = map[rune][]rune{
	'_': []rune("</i>"),
	'*': []rune("</b>"),
	'`': []rune("</code>"),
}

const btnPrefix = "buttonurl:"
const sameLineSuffix = ":same"

var defaultConverter = Converter{
	BtnPrefix:      btnPrefix,
	SameLineSuffix: sameLineSuffix,
}

type Button struct {
	Name     string
	Content  string
	SameLine bool
}

type Converter struct {
	BtnPrefix      string
	SameLineSuffix string
}

func New() *Converter {
	return &Converter{
		BtnPrefix:      btnPrefix,
		SameLineSuffix: sameLineSuffix,
	}
}

func MD2HTML(input string) string {
	text, _ := defaultConverter.md2html([]rune(html.EscapeString(input)), false)
	return text
}

func MD2HTMLButtons(input string) (string, []Button) {
	return defaultConverter.md2html([]rune(html.EscapeString(input)), true)
}

func (cv *Converter) MD2HTML(input string) string {
	text, _ := cv.md2html([]rune(html.EscapeString(input)), false)
	return text
}

func (cv *Converter) MD2HTMLButtons(input string) (string, []Button) {
	return cv.md2html([]rune(html.EscapeString(input)), true)
}

// todo: ``` support? -> add \n char to md chars and hence on \n, skip
func (cv *Converter) md2html(input []rune, buttons bool) (string, []Button) {
	var output strings.Builder
	v := map[rune][]int{}
	var containedMDChars []rune
	escaped := false
	offset := 0
	lastSync := 0
	var newInput []rune
	for pos, char := range input {
		if escaped {
			escaped = false
			continue
		}
		switch char {
		case '_', '*', '`', '[', ']', '(', ')':
			v[char] = append(v[char], pos-offset)
			containedMDChars = append(containedMDChars, char)
		case '\\':
			escaped = true
			newInput = append(newInput, input[lastSync:pos]...)
			offset ++
			lastSync = pos + 1
		}
	}
	input = append(newInput, input[lastSync:]...)

	prev := 0
	var btnPairs []Button
	for i := 0; i < len(containedMDChars) && prev < len(input); i++ {
		currChar := containedMDChars[i]
		posArr := v[currChar]
		// if we're past the currChar position, pass and update
		if posArr[0] < prev {
			v[currChar] = posArr[1:]
			continue
		}
		switch currChar {
		case '_', '*', '`':
			// if fewer than 2 chars left, pass
			if len(posArr) < 2 {
				continue
			}

			bkp := map[rune][]int{}
			cnt := i // copy i to avoid changing if false
			// skip i to next same char (hence jumping all inbetween) (could be done with a normal range and continues?)
			// todo: OOB check on +1?
			for _, val := range containedMDChars[cnt+1:] {
				cnt++
				if val == currChar {
					break
				}

				if x, ok := bkp[val]; ok {
					bkp[val] = x[1:]
				} else {
					bkp[val] = v[val][1:]
				}
			}
			// pop currChar
			fstPos, rest := posArr[0], posArr[1:]
			v[currChar] = rest

			if !validStart(fstPos, input) {
				continue
			}
			ok := false
			var sndPos int
			for _, sndPos = range rest {
				rest = rest[1:]
				if validEnd(sndPos, input) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}

			v[currChar] = rest
			output.WriteString(string(input[prev:fstPos]))
			output.WriteString(string(openHTML[currChar]))
			output.WriteString(string(input[fstPos+1 : sndPos]))
			output.WriteString(string(closeHTML[currChar]))
			prev = sndPos + 1
			i = cnt // set i to copy
			for x, y := range bkp {
				v[x] = y
			}

		case '[':
			nameOpen, rest := posArr[0], posArr[1:]
			v['['] = rest
			if len(v[']']) < 1 || len(v['(']) < 1 || len(v[')']) < 1 || nameOpen < prev {
				continue
			}

			var nextNameClose int
			var nextLinkOpen int
			var nextLinkClose int

			wastedLinkClose := v[')']
		LinkLabel:
			for _, closeLinkPos := range v[')'] {
				if closeLinkPos > nameOpen {
					wastedLinkOpen := v['(']
					for _, openLinkpos := range v['('] {
						if openLinkpos > nameOpen && openLinkpos < closeLinkPos {
							wastedNameClose := v[']']
							for _, closeNamePos := range v[']'] {
								if closeNamePos == openLinkpos-1 {
									nextNameClose = closeNamePos
									nextLinkOpen = openLinkpos
									nextLinkClose = closeLinkPos
									v[']'] = wastedNameClose
									v['('] = wastedLinkOpen
									v[')'] = wastedLinkClose

									break LinkLabel
								}
								wastedNameClose = wastedNameClose[1:]
							}
						}
						wastedLinkOpen = wastedLinkOpen[1:]
					}
				}
				wastedLinkClose = wastedLinkClose[1:]
			}
			if nextLinkClose == 0 {
				// invalid
				continue
			}

			output.WriteString(string(input[prev:nameOpen]))
			link := string(input[nextLinkOpen+1 : nextLinkClose])
			name := string(input[nameOpen+1 : nextNameClose])
			if buttons && strings.HasPrefix(link, cv.BtnPrefix) {
				// is a button
				sameline := strings.HasSuffix(link, cv.SameLineSuffix)
				if sameline {
					link = link[:len(link)-len(cv.SameLineSuffix)]
				}
				btnPairs = append(btnPairs, Button{
					Name:     html.UnescapeString(name),
					Content:  strings.TrimLeft(link[len(cv.BtnPrefix):], "/"),
					SameLine: sameline,
				})
			} else {
				output.WriteString(`<a href="` + link + `">` + name + `</a>`)
			}

			prev = nextLinkClose + 1
		}
	}
	output.WriteString(string(input[prev:]))

	return output.String(), btnPairs
}

func validStart(pos int, input []rune) bool {
	return (pos == 0 || !(unicode.IsLetter(input[pos-1]) || unicode.IsDigit(input[pos-1]))) && !(pos == len(input)-1 || unicode.IsSpace(input[pos+1]))
}

func validEnd(pos int, input []rune) bool {
	return !(pos == 0 || unicode.IsSpace(input[pos-1])) && (pos == len(input)-1 || !(unicode.IsLetter(input[pos+1]) || unicode.IsDigit(input[pos+1])))
}

func Reverse(s string, buttons []Button) string {
	return defaultConverter.Reverse(s, buttons)
}

func (cv *Converter) Reverse(s string, buttons []Button) string {
	return cv.reverse([]rune(s), buttons)
}

var link = regexp.MustCompile(`a href="(.*)"`)

// TODO: this needs to return string, error to handle bad parsing
func (cv *Converter) reverse(r []rune, buttons []Button) string {
	prev := 0
	out := strings.Builder{}
	for i := 0; i < len(r); i++ {
		if r[i] == '<' {
			closeTag := 0
			for ix, c := range r[i+1:] {
				if c == '>' {
					closeTag = i + ix + 1
					break
				}
			}
			if closeTag == 0 {
				// "no close tag"
				return ""
			}

			closingOpen := 0
			for ix, c := range r[closeTag:] {
				if c == '<' {
					closingOpen = closeTag + ix
					break
				}
			}
			if closingOpen == 0 {
				// "no closing open"
				return ""
			}

			closingClose := 0
			for ix, c := range r[closingOpen:] {
				if c == '>' {
					closingClose = closingOpen + ix
					break
				}
			}
			if closingClose == 0 {
				// "no closing close"
				return ""
			}

			tag := string(r[i+1 : closeTag])
			// todo: check expected closing tag
			out.WriteString(string(r[prev:i]))
			if link.MatchString(tag) {
				matches := link.FindStringSubmatch(tag)
				out.WriteString("[" + EscapeMarkdown(r[closeTag+1:closingOpen], []rune{'[', ']', '(', ')'}) + "](" + matches[1] + ")")
			} else if tag == "b" {
				out.WriteString("*" + EscapeMarkdown(r[closeTag+1:closingOpen], []rune{'*'}) + "*")
			} else if tag == "i" {
				out.WriteString("_" + EscapeMarkdown(r[closeTag+1:closingOpen], []rune{'_'}) + "_")
			} else if tag == "code" {
				out.WriteString("`" + EscapeMarkdown(r[closeTag+1:closingOpen], []rune{'`'}) + "`")
			} else {
				// unknown tag
				return ""
			}
			prev = closingClose + 1
			i = closingClose
		}
	}
	out.WriteString(string(r[prev:]))

	for _, btn := range buttons {
		out.WriteString("\n[" + btn.Name + "](buttonurl://" + btn.Content)
		if btn.SameLine {
			out.WriteString(":same")
		}
		out.WriteString(")")
	}

	return out.String()
}

func EscapeMarkdown(r []rune, toEscape []rune) string {
	out := strings.Builder{}
	for i, x := range r {
		if contains(x, toEscape) {
			if i == 0 || i == len(r)-1 || validEnd(i, r) || validStart(i, r) {
				out.WriteRune('\\')
			}
		}
		out.WriteRune(x)
	}
	return out.String()
}

func contains(r rune, rr []rune) bool {
	for _, x := range rr {
		if r == x {
			return true
		}
	}

	return false
}
