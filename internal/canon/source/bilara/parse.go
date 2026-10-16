package bilara

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/lavantien/quotes-manager/internal/canon/model"
	"github.com/lavantien/quotes-manager/internal/canon/norm"
)

type Result struct {
	Records          []model.SegmentRecord
	DupKeys          []string
	NFCChanged       bool
	PrefixMismatches []string
}

type NameFile map[string]string

var (
	bom            = []byte{0xEF, 0xBB, 0xBF}
	errInvalidUTF8 = errors.New("invalid utf-8 in bilara input")
)

func ParseSegments(r io.Reader, stream model.Stream, muid string) (Result, error) {
	var res Result
	seen := make(map[string]bool)
	err := eachEntry(r, func(key, value string) error {
		nkey, keyChanged := norm.NFC(key)
		ntext, valChanged := norm.NFC(value)
		if seen[nkey] {
			res.DupKeys = append(res.DupKeys, nkey)
			return nil
		}
		seen[nkey] = true
		if keyChanged || valChanged {
			res.NFCChanged = true
		}
		keyMuid, segKey, found := strings.Cut(nkey, ":")
		rec := model.SegmentRecord{Stream: stream, Muid: muid, SegKey: nkey, Text: ntext}
		if found {
			rec.Muid = keyMuid
			rec.SegKey = segKey
			if keyMuid != muid {
				res.PrefixMismatches = append(res.PrefixMismatches, nkey)
			}
		} else {
			res.PrefixMismatches = append(res.PrefixMismatches, nkey)
		}
		res.Records = append(res.Records, rec)
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("bilara parse %s: %w", muid, err)
	}
	return res, nil
}

func ParseNames(r io.Reader) (NameFile, error) {
	nf := NameFile{}
	err := eachEntry(r, func(key, value string) error {
		nkey, _ := norm.NFC(key)
		nvalue, _ := norm.NFC(value)
		if _, dup := nf[nkey]; !dup {
			nf[nkey] = nvalue
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("bilara names: %w", err)
	}
	return nf, nil
}

func eachEntry(r io.Reader, fn func(key, value string) error) error {
	br := bufio.NewReader(r)
	if b, _ := br.Peek(3); len(b) == 3 && bytes.Equal(b, bom) {
		if _, err := br.Discard(3); err != nil {
			return err
		}
	}
	dec := json.NewDecoder(&utf8Reader{r: br})

	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return fmt.Errorf("top level must be an object, got %v", tok)
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("object key must be a string, got %v", keyTok)
		}
		valTok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("key %q: %w", key, err)
		}
		val, ok := valTok.(string)
		if !ok {
			return fmt.Errorf("key %q: value must be a string, got %v", key, valTok)
		}
		if err := fn(key, val); err != nil {
			return err
		}
	}
	if _, err := dec.Token(); err != nil {
		return err
	}
	if _, err := dec.Token(); err == nil {
		return errors.New("trailing data after object")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

type utf8Reader struct {
	r    io.Reader
	pend []byte
	tail []byte
	eof  bool
}

func (u *utf8Reader) Read(p []byte) (int, error) {
	for len(u.pend) == 0 {
		if u.eof {
			if len(u.tail) > 0 {
				return 0, errInvalidUTF8
			}
			return 0, io.EOF
		}
		var chunk [4096]byte
		n, err := u.r.Read(chunk[:])
		if err != nil && !errors.Is(err, io.EOF) {
			return 0, err
		}
		if errors.Is(err, io.EOF) {
			u.eof = true
		}
		data := chunk[:n]
		if len(u.tail) > 0 {
			combined := make([]byte, 0, len(u.tail)+n)
			combined = append(combined, u.tail...)
			combined = append(combined, data...)
			u.tail = u.tail[:0]
			data = combined
		}
		cut := len(data)
		if !u.eof {
			cut = completeRunePrefix(data)
		}
		if !utf8.Valid(data[:cut]) {
			return 0, errInvalidUTF8
		}
		u.pend = data[:cut]
		if rest := data[cut:]; len(rest) > 0 {
			u.tail = append(u.tail, rest...)
		}
	}
	n := copy(p, u.pend)
	u.pend = u.pend[n:]
	return n, nil
}

func completeRunePrefix(data []byte) int {
	n := len(data)
	for h := 1; h <= 4 && h <= n; h++ {
		b := data[n-h]
		if b < 0x80 {
			return n
		}
		if b >= 0xC0 {
			if leadRuneLen(b) > h {
				return n - h
			}
			return n
		}
	}
	return n
}

func leadRuneLen(b byte) int {
	switch {
	case b >= 0xF0:
		return 4
	case b >= 0xE0:
		return 3
	default:
		return 2
	}
}
