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

	return decode(b, i+1, v)
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
	c := strings.Index(b[st:], ":")
	c += st // catch up with previous string (if exists)
	if c == -1 {
		return st, fmt.Errorf("malformed string encoding")
	}
	n, err := strconv.Atoi(b[st:c])
	if err != nil {
		return st, err
	}
	if len(b[c+1:c+n]) >= n {
		return st, fmt.Errorf("string length mismatch")
	}
	ind := c + 1 // exclude :
	s := b[ind : ind+n]
	*v = append(*v, s)
	//fmt.Println("append string:", s)
	length := c + n
	return length, nil
}

func decodeList(b string, st int, v *[]interface{}) (i int, err error) {
	s := b[st:]
	l := make([]interface{}, 0)
	for j := 1; j < len(s); {
		if unicode.IsDigit(rune(s[j])) { // string
			c := strings.Index(s[j:], ":")
			nEndInd := j + c
			// fmt.Println("c, nEnd", c, nEndInd)
			n, err := strconv.Atoi(string(s[j:nEndInd]))
			// fmt.Println("n", n)
			if err != nil {
				return st, err
			}
			ind := nEndInd + 1 // skip : and n
			l = append(l, s[ind:ind+n])
			// fmt.Println("appending", s[ind:ind+n])
			j = nEndInd + n + 1
			// fmt.Println("i, j", i, j)
		} else if s[j] == 'i' { // integer
			j++
			ie := strings.Index(s[j:], "e")
			in := s[j : ie+j]
			n, _ := strconv.Atoi(in)
			l = append(l, n)
			// fmt.Println("appending", n)
			j += ie + 1
			// fmt.Println("i, j", i, j)
		} else if s[j] == 'l' {
			j, err = decodeList(b, j, &l)
			if err != nil {
				return st, err
			}
			j++
			i = j
		} else if s[j] == 'e' {
			i = st + j
			// fmt.Println("exiting loop", i, j)
			break
		} else {
			j++
		}
	}
	// fmt.Println("returning to decode() with i", i)
	*v = append(*v, l)
	return i, err
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: decode <string>")
		return
	}
	command := os.Args[1]
	var v []interface{}
	if command == "decode" {
		_, err := decode(os.Args[2], 0, &v)
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, value := range v {
			jsonOutput, err := json.Marshal(value)
			if err != nil {
				fmt.Println(err)
				return
			}
			fmt.Println(string(jsonOutput))
		}
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
