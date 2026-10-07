package compiler

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type tokenKind uint8

const (
	tokenWord tokenKind = iota
	tokenNumber
	tokenString
	tokenSymbol
	tokenEOF
)

type token struct {
	kind tokenKind
	text string
	span Span
}

func asciiLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
func asciiDigit(b byte) bool  { return b >= '0' && b <= '9' }

func lex(source string) ([]token, error) {
	if !utf8.ValidString(source) {
		return nil, diagnostic(Span{}, "source is not valid UTF-8")
	}
	var tokens []token
	position := 0
	for position < len(source) {
		start, b := position, source[position]
		if strings.ContainsRune(" \t\n\r\v\f", rune(b)) {
			position++
			continue
		}
		if strings.HasPrefix(source[position:], "//") {
			for position < len(source) && source[position] != '\n' {
				position++
			}
			continue
		}
		if strings.HasPrefix(source[position:], "/*") {
			end := strings.Index(source[position+2:], "*/")
			if end < 0 {
				return nil, diagnostic(Span{start, start + 2}, "unterminated block comment")
			}
			position += end + 4
			continue
		}
		kind, text := tokenSymbol, ""
		switch {
		case asciiLetter(b) || b == '_':
			position++
			for position < len(source) && (asciiLetter(source[position]) || asciiDigit(source[position]) || source[position] == '_') {
				position++
			}
			kind, text = tokenWord, source[start:position]
		case asciiDigit(b):
			position++
			for position < len(source) && asciiDigit(source[position]) {
				position++
			}
			if position+1 < len(source) && source[position] == '.' && asciiDigit(source[position+1]) {
				position++
				for position < len(source) && asciiDigit(source[position]) {
					position++
				}
			}
			if position < len(source) && (source[position] == 'e' || source[position] == 'E') {
				position++
				if position < len(source) && (source[position] == '+' || source[position] == '-') {
					position++
				}
				digits := position
				for position < len(source) && asciiDigit(source[position]) {
					position++
				}
				if digits == position {
					return nil, diagnostic(Span{start, position}, "expected exponent digits")
				}
			}
			if position < len(source) && (source[position] == 'l' || source[position] == 'L') {
				position++
			}
			kind, text = tokenNumber, source[start:position]
		case b == '"':
			position++
			var value strings.Builder
			closed := false
			for position < len(source) {
				character, size := utf8.DecodeRuneInString(source[position:])
				position += size
				if character == '"' {
					closed = true
					break
				}
				switch character {
				case '\n', '\r':
					return nil, diagnostic(Span{start, position}, "newline in string literal; use \\n")
				case 0:
					return nil, diagnostic(Span{start, position}, "NUL bytes are not supported in strings")
				case '\\':
					if position == len(source) {
						break
					}
					escape, size := utf8.DecodeRuneInString(source[position:])
					position += size
					switch escape {
					case 'n':
						value.WriteByte('\n')
					case 'r':
						value.WriteByte('\r')
					case 't':
						value.WriteByte('\t')
					case '\\', '"':
						value.WriteRune(escape)
					default:
						return nil, diagnostic(Span{position - size - 1, position}, fmt.Sprintf("unsupported escape \\%c", escape))
					}
				default:
					value.WriteRune(character)
				}
			}
			if !closed {
				return nil, diagnostic(Span{start, position}, "unterminated string literal")
			}
			kind, text = tokenString, value.String()
		default:
			pair := ""
			if position+1 < len(source) {
				pair = source[position : position+2]
			}
			switch pair {
			case "==", "!=", "<=", ">=", "&&", "||", "++", "--", "+=", "-=", "*=", "/=", "%=":
				position += 2
				text = pair
			default:
				if !strings.ContainsRune("{}();,.+-*/%=!<>&", rune(b)) {
					character, size := utf8.DecodeRuneInString(source[position:])
					return nil, diagnostic(Span{start, start + size}, fmt.Sprintf("unexpected character '%c'", character))
				}
				position++
				text = source[start:position]
			}
		}
		tokens = append(tokens, token{kind, text, Span{start, position}})
	}
	return append(tokens, token{kind: tokenEOF, span: Span{position, position}}), nil
}
