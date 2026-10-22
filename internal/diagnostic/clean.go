package diagnostic

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Clean removes terminal escape sequences and non-printing controls from CLI diagnostics.
func Clean(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' {
			i++
			if i >= len(s) {
				break
			}
			switch s[i] {
			case '[': // CSI, including SGR colors.
				i++
				for i < len(s) {
					b := s[i]
					i++
					if b >= 0x40 && b <= 0x7e {
						break
					}
				}
			case ']': // OSC, terminated by BEL or ESC backslash.
				i++
				for i < len(s) {
					if s[i] == '\a' {
						i++
						break
					}
					if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '\\' {
						i += 2
						break
					}
					i++
				}
			default:
				i++ // Two-byte escape sequence.
			}
			continue
		}
		r := rune(s[i])
		size := 1
		if r >= 0x80 {
			r, size = utf8.DecodeRuneInString(s[i:])
		}
		if !unicode.IsControl(r) || r == '\n' || r == '\t' {
			out.WriteRune(r)
		}
		i += size
	}
	return strings.TrimSpace(out.String())
}
