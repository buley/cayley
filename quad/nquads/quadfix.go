// +build ignore

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/google/cayley/graph"
)

func main() {
	dec := NewDecoder(os.Stdin)
	for {
		t, err := dec.Unmarshal()
		if err != nil {
			if err == io.EOF {
				return
			}
			log.Println(err)
		}
		if t.Subject[0] == ':' && t.Subject[1] == '/' {
			t.Subject = "<" + t.Subject[1:] + ">"
		} else {
			t.Subject = "_" + t.Subject
		}
		if t.Object[0] == ':' {
			if t.Object[1] == '/' {
				t.Object = "<" + t.Object[1:] + ">"
			} else {
				t.Object = "_" + t.Object
			}
		} else if t.Object[0] == '/' {
			t.Object = "<" + t.Object + ">"
		} else {
			t.Object = fmt.Sprintf(`%q`, t.Object)
		}
		fmt.Printf("%s <%s> %s .\n", t.Subject, t.Predicate, t.Object)
	}
}

// Historical N-Quads parser code.
// -------------------------------

var (
	ErrAbsentSubject   = errors.New("nqauds: absent subject")
	ErrAbsentPredicate = errors.New("nqauds: absent predicate")
	ErrAbsentObject    = errors.New("nqauds: absent object")
	ErrUnterminated    = errors.New("nqauds: unterminated quad")
)

func Parse(str string) (*graph.Triple, error) {
	// Skip leading whitespace.
	str = trimSpace(str)
	// Check for a comment
	if str != "" && str[0] == '#' {
		return nil, nil
	}
	sub, remainder := getTripleComponent(str)
	if sub == "" {
		return nil, ErrAbsentSubject
	}
	str = trimSpace(remainder)
	pred, remainder := getTripleComponent(str)
	if pred == "" {
		return nil, ErrAbsentPredicate
	}
	str = trimSpace(remainder)
	obj, remainder := getTripleComponent(str)
	if obj == "" {
		return nil, ErrAbsentObject
	}
	str = trimSpace(remainder)
	prov, remainder := getTripleComponent(str)
	str = trimSpace(remainder)
	if str != "" && str[0] == '.' {
		return &graph.Triple{sub, pred, obj, prov}, nil
	}
	return nil, ErrUnterminated
}

func isSpace(s uint8) bool {
	return s == ' ' || s == '\t' || s == '\r'
}

func trimSpace(str string) string {
	i := 0
	for i < len(str) && isSpace(str[i]) {
		i += 1
	}
	return str[i:]
}

func getTripleComponent(str string) (head, tail string) {
	if len(str) == 0 {
		return "", str
	}
	if str[0] == '<' {
		return getUriPart(str[1:])
	} else if str[0] == '"' {
		return getQuotedPart(str[1:])
	} else if str[0] == '.' {
		return "", str
	} else {
		// Technically not part of the spec. But we do it anyway for convenience.
		return getUnquotedPart(str)
	}
}

func getUriPart(str string) (head, tail string) {
	i := 0
	for i < len(str) && str[i] != '>' {
		i += 1
	}
	if i == len(str) {
		return "", str
	}
	head = str[0:i]
	return head, str[i+1:]
}

func getQuotedPart(str string) (head, tail string) {
	var (
		i     int
		start int
	)
	for i < len(str) && str[i] != '"' {
		if str[i] == '\\' {
			head += str[start:i]
			switch str[i+1] {
			case '\\':
				head += "\\"
			case 'r':
				head += "\r"
			case 'n':
				head += "\n"
			case 't':
				head += "\t"
			case '"':
				head += "\""
			default:
				return "", str
			}
			i += 2
			start = i
			continue
		}
		i += 1
	}
	if i == len(str) {
		return "", str
	}
	head += str[start:i]
	i += 1
	switch {
	case strings.HasPrefix(str[i:], "^^<"):
		// Ignore type, for now
		_, tail = getUriPart(str[i+3:])
	case str[i] == '@':
		_, tail = getUnquotedPart(str[i+1:])
	default:
		tail = str[i:]
	}

	return head, tail
}

func getUnquotedPart(str string) (head, tail string) {
	var (
		i       int
		initStr = str
		start   int
	)
	for i < len(str) && !isSpace(str[i]) {
		if str[i] == '"' {
			part, remainder := getQuotedPart(str[i+1:])
			if part == "" {
				return part, initStr
			}
			head += str[start:i]
			str = remainder
			i = 0
			start = 0
			head += part
		}
		i += 1
	}
	head += str[start:i]
	return head, str[i:]
}

type Decoder struct {
	r    *bufio.Reader
	line []byte
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r)}
}

func (dec *Decoder) Unmarshal() (*graph.Triple, error) {
	dec.line = dec.line[:0]
	for {
		l, pre, err := dec.r.ReadLine()
		if err != nil {
			return nil, err
		}
		dec.line = append(dec.line, l...)
		if !pre {
			break
		}
	}
	triple, err := Parse(string(dec.line))
	if err != nil {
		return nil, fmt.Errorf("failed to parse %q: %v", dec.line, err)
	}
	if triple == nil {
		return dec.Unmarshal()
	}
	return triple, nil
}
