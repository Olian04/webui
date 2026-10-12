package editorctx

// stripJSONC turns JSON with comments and trailing commas, as a tsconfig.json is written, into
// plain JSON. Comments become spaces, so the byte offsets of an error stay where the author
// sees them. It does not check the result: encoding/json does.
func stripJSONC(src []byte) []byte {
	out := make([]byte, 0, len(src))
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '"':
			// A string runs to its closing quote, and may contain anything, "//" included.
			out = append(out, c)
			for i++; i < len(src); i++ {
				out = append(out, src[i])
				if src[i] == '\\' && i+1 < len(src) {
					i++
					out = append(out, src[i])
					continue
				}
				if src[i] == '"' {
					break
				}
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for ; i < len(src) && src[i] != '\n'; i++ {
				out = append(out, ' ')
			}
			if i < len(src) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			out = append(out, ' ', ' ')
			i += 2
			for ; i < len(src); i++ {
				if src[i] == '*' && i+1 < len(src) && src[i+1] == '/' {
					out = append(out, ' ', ' ')
					i++
					break
				}
				if src[i] == '\n' {
					out = append(out, '\n')
				} else {
					out = append(out, ' ')
				}
			}
		default:
			out = append(out, c)
		}
	}
	return dropTrailingCommas(out)
}

// dropTrailingCommas blanks a comma whose next significant byte closes an object or an array.
func dropTrailingCommas(src []byte) []byte {
	inString := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inString {
			switch c {
			case '\\':
				i++
			case '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case ',':
			for j := i + 1; j < len(src); j++ {
				if src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r' {
					continue
				}
				if src[j] == '}' || src[j] == ']' {
					src[i] = ' '
				}
				break
			}
		}
	}
	return src
}
