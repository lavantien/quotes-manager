package e2e

import (
	"fmt"
	"net/http"
	"reflect"
	"unsafe"
)

func unexportedField(v reflect.Value, name string) reflect.Value {
	f := v.FieldByName(name)
	if !f.IsValid() {
		panic(fmt.Sprintf("field %s missing on %s", name, v.Type()))
	}
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
}

func muxPatterns(m *http.ServeMux) ([]string, error) {
	root := reflect.ValueOf(m).Elem()
	idx := unexportedField(root, "index")
	pats := map[string]bool{}
	collect := func(p reflect.Value) error {
		if p.IsNil() {
			return nil
		}
		str := unexportedField(p.Elem(), "str").String()
		if str == "" {
			return fmt.Errorf("registered pattern with empty string")
		}
		pats[str] = true
		return nil
	}
	segments := unexportedField(idx, "segments")
	iter := segments.MapRange()
	for iter.Next() {
		slice := iter.Value()
		for i := range slice.Len() {
			if err := collect(slice.Index(i)); err != nil {
				return nil, err
			}
		}
	}
	multis := unexportedField(idx, "multis")
	for i := range multis.Len() {
		if err := collect(multis.Index(i)); err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(pats))
	for p := range pats {
		out = append(out, p)
	}
	return out, nil
}
