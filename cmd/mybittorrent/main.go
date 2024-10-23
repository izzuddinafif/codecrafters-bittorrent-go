package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode"

	"crypto/rand"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
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
					key := temp[k].([]byte)

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

func check(e error) error {
	if e != nil {
		return e
	}
	return nil
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

func inspect(data []byte) error {
	v := extractData(data)
	for _, val := range v {
		convertByteToString(&val) // Recursively convert []byte to string
	}
	jsonOutput, err := json.MarshalIndent(v, "", "	")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonOutput))
	return nil
}

func extractData(data []byte) map[string]interface{} {
	v := make([]interface{}, 0)
	i, err := decode(data, 0, &v)
	if err != nil {
		log.Fatal(err)
	}
	if i != len(data) {
		log.Fatal("extra data found after valid bencoding")
	}

	return v[0].(map[string]interface{})
}

func decodeInfo(data []byte) error {
	d := extractData(data)
	fmt.Println("here")
	ann, ok := d["announce"].(string)
	if !ok {
		fmt.Print("Tracker URL: no tracker URL ")
	} else {
		fmt.Print("Tracker URL: ", ann)
	}
	fmt.Println("")

	info, ok := d["info"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("'info' is not a dictionary")
	}
	fmt.Print("Length: ", info["length"])
	fmt.Println("")

	hash, err := hashInfo(data)
	check(err)
	fmt.Printf("Info Hash: %x", hash)
	fmt.Println("")

	fmt.Print("Piece Length: ", info["piece length"])
	fmt.Println("")
	fmt.Printf("Piece Hashes: %x", info["pieces"])
	fmt.Println("")
	return nil
}

func hashInfo(data []byte) (hash []byte, err error) {
	infoIdx := bytes.Index(data, []byte("4:info"))
	if infoIdx == -1 {
		return nil, fmt.Errorf("info key not found in the bencoded data")
	}
	infoIdx += 6
	info := data[infoIdx:]
	e := bytes.LastIndex(info, []byte("e"))
	info = info[:e]
	hasher := sha1.New()
	hasher.Write(info)

	hash = hasher.Sum(nil)
	return hash, nil
}

func trackerGetReq(URL string, data []byte, l int) (map[string]interface{}, error) {
	left := strconv.Itoa(l)
	hash, err := hashInfo(data)
	if err != nil {
		return nil, err
	}
	var (
		peer_id, port, uploaded, downloaded, compact string = "IzzuddinAhmadAfif:-)", "6881", "0", "0", "1"
	)

	trackerURL, _ := url.Parse(URL)
	v := url.Values{}
	v.Add("info_hash", string(hash))
	/*
		URL encoding of binary data based on RFC 3986, see https://datatracker.ietf.org/doc/html/rfc3986#section-2.1

		var encodedInfoHash string
		for _, b := range infoHash {
		    encodedInfoHash += fmt.Sprintf("%%%02X", b)
		}
	*/
	v.Add("peer_id", peer_id)
	v.Add("port", port)
	v.Add("uploaded", uploaded)
	v.Add("downloaded", downloaded)
	v.Add("left", left)
	v.Add("compact", compact)

	trackerURL.RawQuery = v.Encode()
	resp, err := http.Get(trackerURL.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)

	return extractData(b), err
}

func parsePeers(d map[string]interface{}) []string {
	peers := d["peers"].(string)
	var sockets []string
	for i := 0; i < len(peers)-5; {
		ip := net.IP(peers[i : i+4])
		// Port is 2 bytes = 2^(8*2) = 16 bit unsigned integer (see RFC 793 Section 3.1), big-endian format or network byte order (see RFC 1700)
		port := binary.BigEndian.Uint16([]byte(peers[i+4 : i+6]))
		sockets = append(sockets, fmt.Sprint(ip, ":", port))
		i += 6
	}
	return sockets
}

/*
	IPs and Ports to test bittorrent protocol handshake locally:
	165.232.41.73:51556
	165.232.38.164:51532
	165.232.35.114:51437
*/

func bpHandshake(hash []byte, socket string) error {
	conn, err := net.Dial("tcp", socket)
	if err != nil {
		return err
	}
	defer conn.Close()

	b := bpHandshakeMsg(hash)

	_, err = conn.Write(b)
	if err != nil {
		return err
	}

	buf := make([]byte, 68)
	_, err = conn.Read(buf)
	if err != nil {
		return err
	}
	peer_id := buf[48:]
	fmt.Printf("Peer ID: %x\n", peer_id)
	return nil
}

func bpHandshakeMsg(hash []byte) (b []byte) {
	b = append(b, byte(19))
	b = append(b, []byte("BitTorrent protocol")...)
	b = append(b, make([]byte, 8)...)
	b = append(b, hash...)
	buf := make([]byte, 20)
	_, err := rand.Read(buf)
	if err != nil {
		log.Fatal(err)
	}
	b = append(b, buf...)
	return b
}

func runCommand(command string) {
	var err error
	data := readFile(os.Args[2])

	switch command {
	case "decode":
		err = decodeAndPrint([]byte(os.Args[2]))
		check(err)
	case "info":
		err = decodeInfo(data)
		check(err)
	case "inspect":
		err = inspect(data)
		check(err)
	case "peers":
		d := extractData(data)
		info, ok := d["info"].(map[string]interface{})
		if !ok {
			log.Fatal("'info' is not a dictionary")
		}
		d, err = trackerGetReq(d["announce"].(string), data, info["length"].(int))
		check(err)
		peers := parsePeers(d)
		check(err)
		for _, p := range peers {
			fmt.Println(p)
		}
	case "handshake":
		if len(os.Args) < 4 {
			fmt.Println("Usage: handshake <filename> <ip:port>")
			return
		}
		hash, err := hashInfo(data)
		check(err)
		bpHandshake(hash, os.Args[3])
	default:
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: (decode/info/peers/handshake) <string>")
		return
	}
	command := os.Args[1]
	runCommand(command)
}
