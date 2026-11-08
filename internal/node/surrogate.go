package node

import (
	"fmt"
	"unicode/utf16"
)

// rewriteSurrogates rewrites UTF-16 surrogate escapes inside JSON double-quoted
// strings so that the YAML 1.2 scanner accepts them while preserving column
// positions after each string on the same line:
//   - A high surrogate (\uD800..\uDBFF) immediately followed by a low surrogate
//     (\uDC00..\uDFFF) becomes a single 8-digit \UXXXXXXXX escape (saving 2 bytes),
//     and 2 spaces per pair are appended immediately after the string's closing
//     double quote.
//   - A lone high or low surrogate becomes \uFFFD (same 6-byte length).
//   - If no surrogate escapes are found, data is returned unchanged without copying.
func rewriteSurrogates(data []byte) []byte {
	var out []byte
	inString := false
	pad := 0

	for i := 0; i < len(data); {
		b := data[i]
		if !inString {
			if b == '"' {
				inString = true
				pad = 0
			}
			if out != nil {
				out = append(out, b)
			}
			i++
			continue
		}

		// Inside a double-quoted string.
		if b == '"' {
			inString = false
			if out != nil {
				out = append(out, '"')
				for range pad {
					out = append(out, ' ')
				}
			}
			pad = 0
			i++
			continue
		}

		if b == '\\' {
			if r1, ok := parseU4(data[i:]); ok && isSurrogate(r1) {
				if out == nil {
					out = make([]byte, 0, len(data))
					out = append(out, data[:i]...)
				}
				if isHighSurrogate(r1) {
					if r2, ok2 := parseU4(data[i+6:]); ok2 && isLowSurrogate(r2) {
						cp := utf16.DecodeRune(r1, r2)
						out = fmt.Appendf(out, "\\U%08X", cp)
						pad += 2
						i += 12
						continue
					}
				}
				out = append(out, "\\uFFFD"...)
				i += 6
				continue
			}

			// Ordinary escape: consume the backslash and the escaped byte so an
			// escaped backslash (\\) or escaped quote (\") cannot be misread.
			step := 2
			if i+1 >= len(data) {
				step = 1
			}
			if out != nil {
				out = append(out, data[i:i+step]...)
			}
			i += step
			continue
		}

		if out != nil {
			out = append(out, b)
		}
		i++
	}

	if out == nil {
		return data
	}
	return out
}

func parseU4(b []byte) (rune, bool) {
	if len(b) < 6 || b[0] != '\\' || b[1] != 'u' {
		return 0, false
	}
	var r rune
	for j := 2; j < 6; j++ {
		d, ok := unhex(b[j])
		if !ok {
			return 0, false
		}
		r = (r << 4) | d
	}
	return r, true
}

func unhex(b byte) (rune, bool) {
	switch {
	case '0' <= b && b <= '9':
		return rune(b - '0'), true
	case 'a' <= b && b <= 'f':
		return rune(b-'a') + 10, true
	case 'A' <= b && b <= 'F':
		return rune(b-'A') + 10, true
	default:
		return 0, false
	}
}

func isHighSurrogate(r rune) bool { return 0xD800 <= r && r <= 0xDBFF }
func isLowSurrogate(r rune) bool  { return 0xDC00 <= r && r <= 0xDFFF }
func isSurrogate(r rune) bool     { return 0xD800 <= r && r <= 0xDFFF }
