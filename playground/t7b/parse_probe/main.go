package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type member struct {
	Muid    string
	Display string
	Group   int
	Pos     int
}

var (
	sectionRe = regexp.MustCompile(`^\*\*([IVXL]+)\.\s+(.*?)\*\*`)
	linkRe    = regexp.MustCompile(`\[([^\]]*)\]\(https://suttacentral\.net/([^)]*)\)`)
)

func romanToInt(s string) int {
	vals := map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50}
	total := 0
	for i := 0; i < len(s); i++ {
		v := vals[s[i]]
		if i+1 < len(s) && vals[s[i+1]] > v {
			total -= v
		} else {
			total += v
		}
	}
	return total
}

func expandSlug(slug, label string) []string {
	if strings.Contains(slug, "/") {
		return nil
	}
	if !strings.ContainsAny(slug, "0123456789") {
		return nil
	}
	parts := strings.SplitN(slug, "-", 2)
	if len(parts) == 2 && strings.ContainsAny(parts[1], "0123456789") {
		head := parts[0]
		dot := strings.LastIndex(head, ".")
		if dot >= 0 {
			family := head[:dot]
			start, err1 := strconv.Atoi(head[dot+1:])
			end, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || end < start || end-start > 500 {
				return nil
			}
			out := make([]string, 0, end-start+1)
			for n := start; n <= end; n++ {
				out = append(out, fmt.Sprintf("%s.%d", family, n))
			}
			return out
		}
		trim := len(head)
		for trim > 0 && head[trim-1] >= '0' && head[trim-1] <= '9' {
			trim--
		}
		if trim > 0 && trim < len(head) {
			family := head[:trim]
			start, err1 := strconv.Atoi(head[trim:])
			end, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || end < start || end-start > 500 {
				return nil
			}
			out := make([]string, 0, end-start+1)
			for n := start; n <= end; n++ {
				out = append(out, fmt.Sprintf("%s.%d", family, n))
			}
			return out
		}
	}
	return []string{slug}
}

func main() {
	data, err := os.ReadFile("dumps/ariyasavaka-curated-list.md")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	group := 0
	seen := map[string]bool{}
	var members []member
	pos := 0
	skipped := map[string]int{}
	for _, ln := range lines {
		if m := sectionRe.FindStringSubmatch(ln); m != nil {
			group = romanToInt(m[1])
		}
		if group == 0 {
			continue
		}
		for _, m := range linkRe.FindAllStringSubmatch(ln, -1) {
			label := m[1]
			url := m[2]
			slug := strings.SplitN(url, "/", 2)[0]
			slug = strings.SplitN(slug, "?", 2)[0]
			for _, mu := range expandSlug(slug, label) {
				if seen[mu] {
					continue
				}
				seen[mu] = true
				pos++
				members = append(members, member{Muid: mu, Display: label, Group: group, Pos: pos})
			}
			if len(expandSlug(slug, label)) == 0 {
				skipped[slug]++
			}
		}
	}
	byGroup := map[int]int{}
	for _, mm := range members {
		byGroup[mm.Group]++
	}
	keys := make([]int, 0, len(byGroup))
	for k := range byGroup {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	sizes := make([]int, 0, len(keys))
	for _, k := range keys {
		sizes = append(sizes, byGroup[k])
	}
	fmt.Printf("total=%d groups=%d sizes=%v first=%s last=%s\n", len(members), len(keys), sizes, members[0].Muid, members[len(members)-1].Muid)
	var sk []string
	for k := range skipped {
		sk = append(sk, k)
	}
	sort.Strings(sk)
	fmt.Printf("skipped_slugs=%v\n", sk)
	out, _ := json.Marshal(members)
	if err := os.WriteFile("playground/t7b/derived.json", out, 0o644); err != nil {
		panic(err)
	}
}
