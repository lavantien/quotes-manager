package graph

const schemaV1 = `
CREATE TABLE node (id INTEGER PRIMARY KEY, type TEXT NOT NULL, key TEXT NOT NULL, data TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(data)));
CREATE UNIQUE INDEX ux_node ON node(type, key);
CREATE INDEX ix_node_type ON node(type);
CREATE TABLE edge (id INTEGER PRIMARY KEY, src INTEGER NOT NULL REFERENCES node(id), dst INTEGER NOT NULL REFERENCES node(id), type TEXT NOT NULL, ord INTEGER, data TEXT NOT NULL DEFAULT '{}');
CREATE UNIQUE INDEX ux_edge ON edge(src, dst, type);
CREATE INDEX ix_edge_src ON edge(src, type, ord);
CREATE INDEX ix_edge_dst ON edge(dst, type);
CREATE TABLE run (id INTEGER PRIMARY KEY, bilara_commit TEXT, editions_commit TEXT, started TEXT, finished TEXT);
CREATE TABLE discrepancy (id INTEGER PRIMARY KEY, run_id INTEGER NOT NULL REFERENCES run(id), "check" TEXT NOT NULL, severity TEXT NOT NULL CHECK(severity IN ('info','warn','error')), muid TEXT, node_id INTEGER, detail TEXT NOT NULL);
CREATE INDEX ix_disc_run ON discrepancy(run_id, "check", severity);
CREATE VIRTUAL TABLE fts_segment USING fts5(pali, en, tokenize = 'unicode61 remove_diacritics 2', content = '');
`
