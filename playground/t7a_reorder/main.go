package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lavantien/quotes-manager/internal/store"
)

func main() {
	dir, err := os.MkdirTemp("", "t7a-reorder")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	st, err := store.Open(filepath.Join(dir, "q.db"))
	if err != nil {
		panic(err)
	}
	defer func() { _ = st.Close() }()
	cid, err := st.CreateCollection(nil)
	if err != nil {
		panic(err)
	}
	var gids []int64
	for _, n := range []string{"a", "b", "c"} {
		gid, err := st.CreateCollectionGroup(cid, n)
		if err != nil {
			panic(err)
		}
		gids = append(gids, gid)
	}
	err = st.ReorderCollectionGroups(cid, gids[:2])
	fmt.Printf("partial_reorder_err=%v\n", err)
	groups, err := st.ListCollectionGroups(cid)
	if err != nil {
		panic(err)
	}
	for _, g := range groups {
		var pos int
		if err := st.DB().QueryRow("SELECT position FROM collection_groups WHERE id = ?", g.ID).Scan(&pos); err != nil {
			panic(err)
		}
		fmt.Printf("group=%d name=%s stored_pos=%d\n", g.ID, g.Name, pos)
	}
	err = st.ReorderCollectionGroups(cid, gids[2:])
	fmt.Printf("tail_reorder_err=%v\n", err)
	var posA, posC int
	if err := st.DB().QueryRow("SELECT position FROM collection_groups WHERE id = ?", gids[0]).Scan(&posA); err != nil {
		panic(err)
	}
	if err := st.DB().QueryRow("SELECT position FROM collection_groups WHERE id = ?", gids[2]).Scan(&posC); err != nil {
		panic(err)
	}
	fmt.Printf("pos_a=%d pos_c=%d duplicate=%v\n", posA, posC, posA == posC)
	groups, err = st.ListCollectionGroups(cid)
	if err != nil {
		panic(err)
	}
	order := ""
	for _, g := range groups {
		order += fmt.Sprintf("%d", g.ID)
	}
	fmt.Printf("listed_order=%s\n", order)
	err = st.ReorderCollectionGroups(cid, []int64{gids[0], gids[0], gids[1], gids[2]})
	fmt.Printf("dup_reorder_err=%v\n", err)
	groups, err = st.ListCollectionGroups(cid)
	if err != nil {
		panic(err)
	}
	for _, g := range groups {
		fmt.Printf("after_dup group=%d name=%s\n", g.ID, g.Name)
	}
}
