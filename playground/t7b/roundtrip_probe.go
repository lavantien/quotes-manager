package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/lavantien/quotes-manager/internal/store"
)

func dumpEq(a, b *store.Dump) (string, bool) {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	if string(aj) == string(bj) {
		return "", true
	}
	return fmt.Sprintf("len(a)=%d len(b)=%d", len(aj), len(bj)), false
}

func main() {
	src := "playground/t7b/rt.db"
	data, err := os.ReadFile("database/quotes.db")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(src, data, 0o644); err != nil {
		panic(err)
	}
	s, err := store.Open(src)
	if err != nil {
		panic(err)
	}
	defer func() { _ = s.Close() }()
	first, err := s.Export()
	if err != nil {
		panic(err)
	}
	if err := s.Import(first); err != nil {
		panic(err)
	}
	second, err := s.Export()
	if err != nil {
		panic(err)
	}
	if msg, ok := dumpEq(first, second); !ok {
		fmt.Printf("ROUND TRIP MISMATCH %s\n", msg)
		os.Exit(1)
	}
	fmt.Printf("round trip stable: quotes=%d collections=%d suttas=%d\n", len(first.Quotes), len(first.Collections), len(first.Suttas))
	col := first.Collections[0]
	fmt.Printf("collection 1: name=%s quoteItems=%d groups=%d suttaItems=%d\n", col.Name, len(col.Items), len(col.Groups), len(col.SuttaItems))
	known := 0
	for _, su := range first.Suttas {
		if su.Length > 0 {
			known++
		}
	}
	fmt.Printf("suttas with length=%d\n", known)
	v1 := &store.Dump{Version: 1, Quotes: first.Quotes, Collections: []store.CollectionDump{{
		ID: col.ID, Name: col.Name, Items: col.Items,
	}}, Categories: first.Categories}
	if err := s.Import(v1); err != nil {
		panic(err)
	}
	after, err := s.Export()
	if err != nil {
		panic(err)
	}
	fmt.Printf("v1 import: version=%d collections=%d groups=%d suttas=%d quotes=%d\n", after.Version, len(after.Collections), len(after.Collections[0].Groups), len(after.Suttas), len(after.Quotes))
	bad := &store.Dump{Version: 3}
	if err := s.Import(bad); err != store.ErrUnsupportedDump {
		fmt.Printf("v3 dump accepted: err=%v\n", err)
		os.Exit(1)
	}
	if err := s.Import(nil); err != store.ErrUnsupportedDump {
		fmt.Printf("nil dump accepted: err=%v\n", err)
		os.Exit(1)
	}
	fmt.Println("v3 and nil dumps rejected")
}
