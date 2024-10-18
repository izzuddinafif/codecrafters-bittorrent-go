package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"
)

func decode(b string, st int, v *[]interface{}) (i int, err error) {
	if st == len(b) {
		return st, io.ErrUnexpectedEOF
	}
	i = st

	switch {
	case b[i] == 'i':
		i, err = decodeInt(b, i, v)
		if err != nil {
			return st, err
		}
		return decodeNext(b, i, v)
	case unicode.IsDigit(rune(b[i])):
		i, err := decodeStr(b, i, v)
		if err != nil {
			return st, err
		}
		return decodeNext(b, i, v)
	case b[i] == 'l':
		i, err = decodeList(b, i, v)
		if err != nil {
			return st, err
		}
		return decodeNext(b, i, v)
	default:
		return st, fmt.Errorf("unexpected value: %q, i: %d", b[i], i)
	}
}

func decodeNext(b string, i int, v *[]interface{}) (int, error) {
	if i+1 >= len(b) {
		return i, nil
	} // exit condition
	remaining := b[i+1:]
	if !isValidBencodeCharacter(remaining[0]) {
		return i, fmt.Errorf("extra data after valid bencoded structure: %q", remaining[0])
	}

	return decode(b, i+1, v)
}
func isValidBencodeCharacter(ch byte) bool {
	return unicode.IsDigit(rune(ch)) || ch == 'i' || ch == 'l' || ch == 'd' || ch == 'e'
}
func decodeInt(b string, st int, v *[]interface{}) (i int, err error) {
	i = st + 1
	if i == len(b) {
		return st, fmt.Errorf("bad int")
	}
	e := strings.Index(b[st:], "e")
	if e == -1 {
		return st, fmt.Errorf("malformed integer encoding")
	}
	e += st
	n := b[i:e]
	if n == "-0" {
		return st, fmt.Errorf("-0 is not allowed")
	}
	if strings.HasPrefix(n, "0") && len(n) > 1 {
		return st, fmt.Errorf("leading 0 is not allowed")
	}
	x, err := strconv.Atoi(n)
	if err != nil {
		return st, err
	}
	*v = append(*v, x)
	//fmt.Println("append int:", x)
	return e, nil
}

func decodeStr(b string, st int, v *[]interface{}) (i int, err error) {
	s := b[st:]
	c := strings.Index(s, ":")
	// c += st // catch up with previous string (if exists)
	if c == -1 {
		return st, fmt.Errorf("malformed string encoding")
	}
	n, err := strconv.Atoi(s[:c])
	if err != nil {
		return st, err
	}
	if len(s) < c+1+n {
		return st, fmt.Errorf("string length mismatch or out of bounds")
	}
	ind := c + 1 // exclude :
	str := s[ind : ind+n]
	*v = append(*v, str)
	//fmt.Println("append string:", str)
	length := c + n + st
	return length, nil
}

func decodeList(b string, st int, v *[]interface{}) (i int, err error) {
	s := b[st:]
	l := make([]interface{}, 0)
	for j := 1; j < len(s); {
		switch {
		case unicode.IsDigit(rune(s[j])):
			str := s[j:]
			c := strings.Index(str, ":")
			j += c
			// fmt.Println("c, nEnd", c, nEndInd)
			n, err := strconv.Atoi(string(str[:c]))
			// fmt.Println("n", n)
			if err != nil {
				return st, err
			}
			ind := c + 1 // skip : and n
			l = append(l, str[ind:ind+n])
			// fmt.Println("appending", str[ind:ind+n])
			j += n + 1
		case s[j] == 'i':
			j++
			ie := strings.Index(s[j:], "e")
			in := s[j : ie+j]
			n, err := strconv.Atoi(in)
			if err != nil {
				return st, err
			}
			l = append(l, n)
			// fmt.Println("appending", n)
			j += ie + 1
		case s[j] == 'l':
			new, err := decodeList(b, st+j, &l)
			if err != nil {
				return st, err
			}
			j = new - st
		case s[j] == 'e':
			i = st + j
			*v = append(*v, l)
			return i + 1, err
		default:
			j++
		}
	}
	return i, fmt.Errorf("'e' not found, malformed list")
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: decode <string>")
		return
	}
	command := os.Args[1]
	v := make([]interface{}, 0)
	if command == "decode" {
		i, err := decode(os.Args[2], 0, &v)
		if err != nil {
			fmt.Println(err)
			return
		}
		// After decoding, ensure no trailing data is left
		if i != len(os.Args[2]) {
			fmt.Println(fmt.Errorf("extra data found after valid bencoding"))
			return
		}

		jsonOutput, err := json.Marshal(v)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println(string(jsonOutput))
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
