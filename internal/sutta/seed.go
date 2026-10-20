package sutta

import (
	"fmt"
	"strings"
)

func sqlString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func RenderSeedSQL(tables []Table, collectionName string) string {
	var b strings.Builder
	b.WriteString("DELETE FROM suttas;\n")
	b.WriteString("DELETE FROM collection_sutta_items;\n")
	b.WriteString("DELETE FROM collection_groups;\n")
	b.WriteString("DELETE FROM collections;\n")
	registrySeen := make(map[string]bool)
	for _, t := range tables {
		for _, g := range t.Groups {
			for _, s := range g.Suttas {
				if registrySeen[s.Muid] {
					continue
				}
				registrySeen[s.Muid] = true
				fmt.Fprintf(&b, "INSERT INTO suttas (muid, display, link) VALUES (%s, %s, %s);\n",
					sqlString(s.Muid), sqlString(s.Display), sqlString(s.Link))
			}
		}
	}
	fmt.Fprintf(&b, "INSERT INTO collections (id, name) VALUES (1, %s);\n", sqlString(collectionName))
	groupID := 0
	for _, t := range tables {
		for _, g := range t.Groups {
			groupID++
			fmt.Fprintf(&b, "INSERT INTO collection_groups (id, collection_id, name, position) VALUES (%d, 1, %s, %d);\n",
				groupID, sqlString(g.Name), groupID)
		}
	}
	itemSeen := make(map[string]bool)
	pos := 0
	gid := 0
	for _, t := range tables {
		for _, g := range t.Groups {
			gid++
			for _, s := range g.Suttas {
				if itemSeen[s.Muid] {
					continue
				}
				itemSeen[s.Muid] = true
				pos++
				fmt.Fprintf(&b, "INSERT INTO collection_sutta_items (collection_id, muid, position, group_id) VALUES (1, %s, %d, %d);\n",
					sqlString(s.Muid), pos, gid)
			}
		}
	}
	return b.String()
}
