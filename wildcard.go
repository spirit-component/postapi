package postapi

import "strings"

const toLower = 'a' - 'A'

type converter func(string) string

type wildcard struct {
	prefix string
	suffix string
}

func (w wildcard) match(s string) bool {
	return len(s) >= len(w.prefix+w.suffix) && strings.HasPrefix(s, w.prefix) && strings.HasSuffix(s, w.suffix)
}

// convert converts a list of string using the passed converter function
func convert(s []string, c converter) []string {
	out := []string{}
	for _, i := range s {
		out = append(out, c(i))
	}
	return out
}

// parseHeaderList tokenize + normalize a string containing a list of headers
func parseHeaderList(headerList string) []string {
	l := len(headerList)
	h := make([]byte, 0, l)
	upper := true
	// Estimate the number headers in order to allocate the right splice size
	t := 0
	for i := 0; i < l; i++ {
		if headerList[i] == ',' {
			t++
		}
	}
	headers := make([]string, 0, t)
	for i := 0; i < l; i++ {
		b := headerList[i]
		if b >= 'a' && b <= 'z' {
			if upper {
				h = append(h, b-toLower)
			} else {
				h = append(h, b)
			}
		} else if b >= 'A' && b <= 'Z' {
			if !upper {
				h = append(h, b+toLower)
			} else {
				h = append(h, b)
			}
		} else if b == '-' || b == '_' || (b >= '0' && b <= '9') {
			h = append(h, b)
		}

		if b == ' ' || b == ',' || i == l-1 {
			if len(h) > 0 {
				// Flush the found header
				headers = append(headers, string(h))
				h = h[:0]
				upper = true
			}
		} else {
			upper = b == '-' || b == '_'
		}
	}
	return headers
}

func wildcardMatchFunc(allowedOrigins []string) func(string) bool {

	allowedWOrigins := []wildcard{}
	allowedOriginsAll := false

	for _, origin := range allowedOrigins {
		origin = strings.ToLower(origin)
		if origin == "*" {
			allowedOriginsAll = true
			allowedWOrigins = nil
			break
		} else if i := strings.IndexByte(origin, '*'); i >= 0 {
			w := wildcard{origin[0:i], origin[i+1:]}
			allowedWOrigins = append(allowedWOrigins, w)
		}
	}

	return func(origin string) bool {
		if allowedOriginsAll {
			return true
		}

		for _, w := range allowedWOrigins {
			if w.match(origin) {
				return true
			}
		}

		return false
	}
}
