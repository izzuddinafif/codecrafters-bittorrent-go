package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"
)

func decode(b []byte, st int, v *[]interface{}) (i int, err error) {
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
	case b[i] == 'd':
		i, err := decodeDict(b, i, v)
		if err != nil {
			return st, err
		}
		return decodeNext(b, i, v)
	default:
		return st, fmt.Errorf("unexpected value: %q, i: %d", b[i], i)
	}
}

func decodeDict(b []byte, st int, v *[]interface{}) (i int, err error) {
	s := b[st:]
	d := make(map[string]any, 0)
	temp := make([]interface{}, 0)
	for j := 1; j < len(s); {
		switch {
		case s[j] == 'e':
			if len(temp)%2 != 0 {
				return st, fmt.Errorf("the key-value pair is not complete")
			}
			for k := 0; k < len(temp)-1; {
				if k%2 == 0 {
					key, _ := temp[k].([]byte)

					keyStr := string(key)

					// Handle the value depending on its type
					switch val := temp[k+1].(type) {
					case []byte: // For string and binary data
						if keyStr == "pieces" {
							d[keyStr] = val
						} else {
							d[keyStr] = string(val) // Treat it as a plain string
						}
					case int: // For integer values
						d[keyStr] = val
					case map[string]interface{}: // Recursively handle nested dictionaries
						d[keyStr] = val
					case []interface{}:
						d[keyStr] = val
					default:
						return st, fmt.Errorf("unexpected value type %T for key %s", v, keyStr)
					}
					// fmt.Println(temp, d)

				}
				k += 2
			}
			*v = append(*v, d)
			i = st + j
			return i + 1, err
		case unicode.IsDigit(rune(s[j])):
			newIdx, err := decodeStr(b, st+j, &temp)
			if err != nil {
				return st, err
			}
			j = newIdx - st
		case s[j] == 'i':
			newIdx, err := decodeInt(b, st+j, &temp)
			if err != nil {
				return st, err
			}
			j = newIdx - st
		case s[j] == 'l':
			newIdx, err := decodeList(b, st+j, &temp)
			if err != nil {
				return st, err
			}
			j = newIdx - st
		case s[j] == 'd':
			newIdx, err := decodeDict(b, st+j, &temp)
			if err != nil {
				return st, err
			}
			j = newIdx - st
		}
	}
	return i, fmt.Errorf("'e' not found, malformed dict")
}

func decodeList(b []byte, st int, v *[]interface{}) (i int, err error) {
	s := b[st:]
	l := make([]interface{}, 0)
	for j := 1; j < len(s); {
		switch {
		case s[j] == 'e':
			i = st + j
			*v = append(*v, l)
			// fmt.Println("appending list:", l)
			return i + 1, err
		case unicode.IsDigit(rune(s[j])):
			newIdx, err := decodeStr(b, st+j, &l)
			if err != nil {
				return st, err
			}
			j = newIdx - st
		case s[j] == 'i':
			newIdx, err := decodeInt(b, st+j, &l)
			if err != nil {
				return st, err
			}
			j = newIdx - st
		case s[j] == 'l':
			newIdx, err := decodeList(b, st+j, &l)
			if err != nil {
				return st, err
			}
			j = newIdx - st
		case s[j] == 'd':
			newIdx, err := decodeDict(b, st+j, &l)
			if err != nil {
				return st, err
			}
			j = newIdx - st
		}
	}
	return i, fmt.Errorf("'e' not found, malformed list")
}

func decodeStr(b []byte, st int, v *[]interface{}) (i int, err error) {
	s := b[st:]
	c := strings.Index(string(s), ":")
	if c == -1 {
		return st, fmt.Errorf("malformed string encoding")
	}
	n, err := strconv.Atoi(string(s[:c]))
	if err != nil {
		return st, err
	}
	if len(s) < c+1+n {
		return st, fmt.Errorf("string length mismatch or out of bounds")
	}
	ind := c + 1 // exclude :
	str := s[ind : ind+n]
	*v = append(*v, str)
	// fmt.Println("append string:", str)
	length := n + st + c
	return length + 1, nil
}

func decodeInt(b []byte, st int, v *[]interface{}) (i int, err error) {
	i = st + 1
	if i == len(b) {
		return st, fmt.Errorf("bad int")
	}
	e := strings.Index(string(b[st:]), "e")
	if e == -1 {
		return st, fmt.Errorf("malformed integer encoding")
	}
	e += st
	n := string(b[i:e])
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
	// fmt.Println("append int:", x)
	return e + 1, nil
}

func decodeNext(b []byte, i int, v *[]interface{}) (int, error) {
	if i >= len(b) {
		// fmt.Println(i, len(b), v)
		return i, nil
	} // exit condition
	remaining := b[i:]
	if !isValidBencodeCharacter(remaining[0]) {
		// fmt.Println(i, len(b), v)
		return i, fmt.Errorf("extra data after valid bencoded structure: %q", remaining[0])
	}
	return decode(b, i, v)
}

func isValidBencodeCharacter(ch byte) bool {
	return unicode.IsDigit(rune(ch)) || ch == 'i' || ch == 'l' || ch == 'd' || ch == 'e'
}

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func readFile(filename string) []byte {
	file, err := os.Open(filename)
	check(err)
	defer file.Close()

	data, err := io.ReadAll(file)
	check(err)

	return data
}

func decodeAndPrint(data []byte) error {
	v := make([]interface{}, 0)
	i, err := decode(data, 0, &v)
	if err != nil {
		return err
	}
	if i != len(data) {
		return fmt.Errorf("extra data found after valid bencoding")
	}
	for _, val := range v {
		convertByteToString(&val) // Recursively convert []byte to string
		jsonOutput, err := json.Marshal(val)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonOutput))
	}
	return nil
}

// Recursive function to convert []byte to string
func convertByteToString(val *interface{}) {
	switch v := (*val).(type) {
	case []byte:
		*val = string(v) // Convert []byte to string
	case []interface{}:
		for i := range v {
			convertByteToString(&v[i]) // Recursively convert elements inside lists
		}
	case map[string]interface{}:
		for key, elem := range v {
			convertByteToString(&elem) // Recursively convert elements inside dictionaries
			v[key] = elem
		}
	}
}

func decodeInfo(data []byte) error {
	v := make([]interface{}, 0)
	i, err := decode(data, 0, &v)
	if err != nil {
		return err
	}
	if i != len(data) {
		return fmt.Errorf("extra data found after valid bencoding")
	}
	d := v[0].(map[string]interface{})

	ann, ok := d["announce"].(string)
	if !ok {
		fmt.Println("Tracker URL: no tracker URL ")
	} else {
		fmt.Println("Tracker URL: ", ann)
	}
	info := d["info"].(map[string]interface{})
	fmt.Print("Length: ", info["length"])
	return nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: (decode/info) <string>")
		return
	}
	command := os.Args[1]
	switch command {
	case "decode":
		err := decodeAndPrint([]byte(os.Args[2]))
		check(err)
	case "info":
		data := readFile(os.Args[2])
		err := decodeInfo(data)
		check(err)
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
