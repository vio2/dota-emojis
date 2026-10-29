package meta

import (
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

type node map[string]any

func Key(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSuffix(name, ".png")
	return strings.TrimSuffix(name, "_png")
}

// Load parses one emoticons list and returns ms_per_frame by Key(image_name).
func Load(path string) (map[string]int, error) {
	list, err := parseList(path)
	if err != nil {
		return nil, err
	}
	if list == nil {
		return nil, fmt.Errorf("%s: no \"emoticons\" block", path)
	}

	return timings(list), nil
}

func LoadDir(root string) (map[string]int, error) {
	result := make(map[string]int)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".txt") {
			return err
		}

		list, err := parseList(path)
		if err != nil || list == nil {
			return err
		}

		for key, ms := range timings(list) {
			if prev, ok := result[key]; ok && prev != ms {
				return fmt.Errorf("%s: %s is %d ms, was %d ms in an earlier list", path, key, ms, prev)
			}
			result[key] = ms
		}
		return nil
	})

	return result, err
}

func parseList(path string) (node, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	toks := tokenize(decode(raw))
	root, pos, err := parsePairs(toks, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if pos != len(toks) {
		return nil, fmt.Errorf("%s: unexpected closing brace", path)
	}

	list, _ := root["emoticons"].(node)
	return list, nil
}

func timings(list node) map[string]int {
	result := make(map[string]int)
	for _, v := range list {
		entry, ok := v.(node)
		if !ok {
			continue
		}
		name, ok := entry["image_name"].(string)
		if !ok {
			continue
		}
		ms, err := strconv.Atoi(fmt.Sprint(entry["ms_per_frame"]))
		if err != nil || ms <= 0 {
			continue
		}
		result[Key(name)] = ms
	}

	return result
}

func decode(raw []byte) string {
	switch {
	case len(raw) >= 2 && raw[0] == 0xFF && raw[1] == 0xFE:
		return decode16(raw[2:], binary.LittleEndian)
	case len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF:
		return decode16(raw[2:], binary.BigEndian)
	case len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF:
		return string(raw[3:])
	}
	return string(raw)
}

func decode16(raw []byte, order binary.ByteOrder) string {
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = order.Uint16(raw[i*2:])
	}
	return string(utf16.Decode(units))
}

type token struct {
	text  string
	quote bool
}

func tokenize(s string) []token {
	var toks []token
	rs := []rune(s)

	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '/' && i+1 < len(rs) && rs[i+1] == '/':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
		case r == '{' || r == '}':
			toks = append(toks, token{text: string(r)})
			i++
		case r == '"':
			i++
			var sb strings.Builder
			for i < len(rs) && rs[i] != '"' {
				if rs[i] == '\\' && i+1 < len(rs) {
					i++
				}
				sb.WriteRune(rs[i])
				i++
			}
			i++
			toks = append(toks, token{text: sb.String(), quote: true})
		default:
			start := i
			for i < len(rs) && !unicode.IsSpace(rs[i]) && rs[i] != '{' && rs[i] != '}' && rs[i] != '"' {
				i++
			}
			toks = append(toks, token{text: string(rs[start:i]), quote: true})
		}
	}

	return toks
}

func parsePairs(toks []token, pos int) (node, int, error) {
	n := node{}

	for pos < len(toks) && toks[pos].text != "}" {
		key := toks[pos]
		if !key.quote {
			return nil, pos, fmt.Errorf("token %d: expected key, got %q", pos, key.text)
		}
		pos++
		if pos >= len(toks) {
			return nil, pos, fmt.Errorf("key %q has no value", key.text)
		}

		val := toks[pos]
		pos++
		name := strings.ToLower(key.text)

		if val.quote {
			n[name] = val.text
			continue
		}
		if val.text != "{" {
			return nil, pos, fmt.Errorf("key %q: unexpected %q", key.text, val.text)
		}

		child, next, err := parsePairs(toks, pos)
		if err != nil {
			return nil, next, err
		}
		if next >= len(toks) {
			return nil, next, fmt.Errorf("key %q: block is never closed", key.text)
		}
		n[name] = child
		pos = next + 1
	}

	return n, pos, nil
}
