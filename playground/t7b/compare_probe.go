package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type mine struct {
	Muid    string `json:"Muid"`
	Display string `json:"Display"`
	Group   int    `json:"Group"`
	Pos     int    `json:"Pos"`
}

type theirs struct {
	Muid      string `json:"muid"`
	Display   string `json:"display"`
	Link      string `json:"link"`
	Length    int    `json:"length"`
	GroupID   int    `json:"group_id"`
	GroupName string `json:"group_name"`
	GroupPos  int    `json:"group_position"`
}

func main() {
	a, err := os.ReadFile("playground/t7b/derived.json")
	if err != nil {
		panic(err)
	}
	b, err := os.ReadFile("playground/t7b/impl.json")
	if err != nil {
		panic(err)
	}
	var ms []mine
	var ts []theirs
	if err := json.Unmarshal(a, &ms); err != nil {
		panic(err)
	}
	if err := json.Unmarshal(b, &ts); err != nil {
		panic(err)
	}
	mineByMuid := map[string]mine{}
	for _, m := range ms {
		mineByMuid[m.Muid] = m
	}
	theirsByMuid := map[string]theirs{}
	for _, t := range ts {
		theirsByMuid[t.Muid] = t
	}
	onlyMine, onlyTheirs := 0, 0
	groupMismatch := 0
	for muid, m := range mineByMuid {
		t, ok := theirsByMuid[muid]
		if !ok {
			onlyMine++
			if onlyMine <= 5 {
				fmt.Printf("only-mine %s\n", muid)
			}
			continue
		}
		if m.Group != t.GroupID {
			groupMismatch++
			if groupMismatch <= 10 {
				fmt.Printf("group-mismatch %s mine=%d impl=%d\n", muid, m.Group, t.GroupID)
			}
		}
	}
	for muid := range theirsByMuid {
		if _, ok := mineByMuid[muid]; !ok {
			onlyTheirs++
			if onlyTheirs <= 5 {
				fmt.Printf("only-impl %s\n", muid)
			}
		}
	}
	labelDiffs := 0
	for _, t := range ts {
		m := mineByMuid[t.Muid]
		if m.Display != t.Display {
			labelDiffs++
			if labelDiffs <= 15 {
				fmt.Printf("display %s label=%q impl=%q\n", t.Muid, m.Display, t.Display)
			}
		}
	}
	names := map[int]string{}
	for _, t := range ts {
		if _, ok := names[t.GroupID]; !ok {
			names[t.GroupID] = t.GroupName
		}
	}
	fmt.Printf("mine=%d impl=%d onlyMine=%d onlyImpl=%d groupMismatch=%d displayDiffs=%d\n", len(mineByMuid), len(theirsByMuid), onlyMine, onlyTheirs, groupMismatch, labelDiffs)
	for i := 1; i <= 16; i++ {
		fmt.Printf("group %d: %s\n", i, names[i])
	}
}
