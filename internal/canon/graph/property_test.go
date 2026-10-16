package graph

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"

	"github.com/lavantien/quotes-manager/internal/canon/model"
)

func TestRandomForestRoundTrip(t *testing.T) {
	const (
		nodes     = 200
		batchSize = 7
		rounds    = 5
	)
	for round := range rounds {
		r := rand.New(rand.NewSource(int64(round) + 1))
		s := openTempStore(t)
		l := newTestLoader(t, s, batchSize)

		parent := make([]int, nodes)
		parent[0] = -1
		ordOf := make([]int, nodes)
		childCount := make([]int, nodes)
		for i := 1; i < nodes; i++ {
			p := r.Intn(i+1) - 1
			parent[i] = p
			if p >= 0 {
				ordOf[i] = childCount[p]
				childCount[p]++
			}
		}

		key := func(i int) string { return fmt.Sprintf("n%03d", i) }
		ids := make([]int64, nodes)
		for i := range nodes {
			id, err := l.EnsureNode(model.NodeDivision, key(i), fmt.Sprintf(`{"v":%d}`, i))
			if err != nil {
				t.Fatal(err)
			}
			ids[i] = id
			if parent[i] >= 0 {
				o := int64(ordOf[i])
				if err := l.AddEdge(ids[i], ids[parent[i]], model.EdgeChildOf, &o, "{}"); err != nil {
					t.Fatal(err)
				}
			}
			if r.Intn(3) == 0 {
				if err := l.Flush(); err != nil {
					t.Fatal(err)
				}
			}
		}
		for range 10 {
			i := r.Intn(nodes)
			id, err := l.EnsureNode(model.NodeDivision, key(i), fmt.Sprintf(`{"v":%d}`, i))
			if err != nil {
				t.Fatal(err)
			}
			if id != ids[i] {
				t.Fatalf("round %d: upsert id for %s = %d, want %d", round, key(i), id, ids[i])
			}
		}
		if err := l.Flush(); err != nil {
			t.Fatal(err)
		}

		var n int
		if err := s.DB().QueryRow(`SELECT COUNT(*) FROM node`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != nodes {
			t.Fatalf("round %d: node count = %d, want %d", round, n, nodes)
		}
		rows, err := s.DB().Query(`SELECT key, data FROM node`)
		if err != nil {
			t.Fatal(err)
		}
		dataByKey := map[string]string{}
		for rows.Next() {
			var k, d string
			if err := rows.Scan(&k, &d); err != nil {
				t.Fatal(err)
			}
			dataByKey[k] = d
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		_ = rows.Close()
		if len(dataByKey) != nodes {
			t.Fatalf("round %d: distinct keys = %d, want %d", round, len(dataByKey), nodes)
		}

		rows, err = s.DB().Query(`
			SELECT c.key, p.key, e.ord
			FROM edge e
			JOIN node c ON c.id = e.src
			JOIN node p ON p.id = e.dst
			WHERE e.type = 'CHILD_OF'`)
		if err != nil {
			t.Fatal(err)
		}
		parentOf := map[string]string{}
		ordsByParent := map[string][]int{}
		var edgeCount int
		for rows.Next() {
			var ck, pk string
			var ord *int
			if err := rows.Scan(&ck, &pk, &ord); err != nil {
				t.Fatal(err)
			}
			edgeCount++
			parentOf[ck] = pk
			if ord == nil {
				t.Fatalf("round %d: edge %s->%s has nil ord", round, ck, pk)
			}
			ordsByParent[pk] = append(ordsByParent[pk], *ord)
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		_ = rows.Close()

		roots := 0
		for i := range nodes {
			if parent[i] < 0 {
				roots++
				continue
			}
			if parentOf[key(i)] != key(parent[i]) {
				t.Fatalf("round %d: parent of %s = %s, want %s",
					round, key(i), parentOf[key(i)], key(parent[i]))
			}
		}
		if edgeCount != nodes-roots {
			t.Fatalf("round %d: edge count = %d, want %d", round, edgeCount, nodes-roots)
		}

		for pk, ords := range ordsByParent {
			sort.Ints(ords)
			for i, o := range ords {
				if o != i {
					t.Fatalf("round %d: ords of %s = %v, want dense 0..%d", round, pk, ords, len(ords)-1)
				}
			}
		}

		for k := range dataByKey {
			cur, steps := k, 0
			for {
				p, ok := parentOf[cur]
				if !ok {
					break
				}
				cur = p
				steps++
				if steps > nodes {
					t.Fatalf("round %d: cycle reached from %s", round, k)
				}
			}
		}
	}
}
