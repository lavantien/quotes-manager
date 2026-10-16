package e2e

import "strings"

type htmlTag struct {
	name  string
	attrs map[string]string
}

func isTagSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func scanHTMLTags(doc string) []htmlTag {
	var tags []htmlTag
	for i := 0; i < len(doc); {
		lt := strings.IndexByte(doc[i:], '<')
		if lt < 0 {
			break
		}
		start := i + lt
		gt := strings.IndexByte(doc[start:], '>')
		if gt < 0 {
			break
		}
		body := doc[start+1 : start+gt]
		i = start + gt + 1
		if body == "" || body[0] == '!' {
			continue
		}
		if name, attrs, ok := parseTagBody(body); ok {
			tags = append(tags, htmlTag{name: name, attrs: attrs})
		}
	}
	return tags
}

func parseTagBody(body string) (string, map[string]string, bool) {
	i := 0
	skipSpace := func() {
		for i < len(body) && isTagSpace(body[i]) {
			i++
		}
	}
	skipSpace()
	nameStart := i
	for i < len(body) && !isTagSpace(body[i]) {
		i++
	}
	if i == nameStart {
		return "", nil, false
	}
	name := strings.ToLower(body[nameStart:i])
	attrs := map[string]string{}
	for {
		skipSpace()
		if i >= len(body) {
			break
		}
		keyStart := i
		for i < len(body) && body[i] != '=' && !isTagSpace(body[i]) {
			i++
		}
		if i == keyStart {
			i++
			continue
		}
		key := strings.ToLower(body[keyStart:i])
		val := ""
		if i < len(body) && body[i] == '=' {
			i++
			if i < len(body) && (body[i] == '"' || body[i] == '\'') {
				quote := body[i]
				i++
				valStart := i
				for i < len(body) && body[i] != quote {
					i++
				}
				val = body[valStart:i]
				if i < len(body) {
					i++
				}
			} else {
				valStart := i
				for i < len(body) && !isTagSpace(body[i]) {
					i++
				}
				val = body[valStart:i]
			}
		}
		attrs[key] = val
	}
	return name, attrs, true
}
