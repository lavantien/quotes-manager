import re
text = open("dumps/ariyasavaka-curated-list.md", encoding="utf-8").read()
seed = open("database/seed.sql", encoding="utf-8").read()
headings = list(re.finditer(r"^\*\*([IVXLCDM]+)\.\s*(.+?)\*\*", text, re.M))
families = {"mn", "dn", "an", "sn", "ud", "iti", "snp", "dhp", "khp", "vv", "ja", "thag", "thig"}
def norm(s):
    return ".".join(p.lstrip("0") or "0" for p in s.split("."))
def expand(slug):
    m = re.match(r"^([a-z]+?)(\d+(?:\.\d+)?)-(\d+(?:\.\d+)?)$", slug)
    if not m:
        return [slug]
    fam, a, b = m.group(1), norm(m.group(2)), norm(m.group(3))
    ca = na = cb = None
    if "." in a: ca, na = a.split(".")
    else: na = a
    if "." in b: cb, nb = b.split(".")
    else: nb = b
    if ca is None: ca = cb
    if cb is None: cb = ca
    if ca != cb: return [slug]
    lo, hi = int(na), int(nb)
    if "." in a and "." in b:
        return [f"{fam}{ca}.{n}" for n in range(lo, hi + 1)]
    return [f"{fam}{n}" for n in range(lo, hi + 1)]
seen = set()
mine = {}
for i, hm in enumerate(headings):
    end = headings[i + 1].end() if i + 1 < len(headings) else len(text)
    for lm in re.finditer(r"\[([^\]]*)\]\(([^()\s]+)\)", text[hm.end():end]):
        url = lm.group(2).replace("&amp;", "&")
        hp = re.sub(r"^[a-z]+://", "", url)
        if "suttacentral" not in hp: continue
        sm = re.match(r"^[a-z.]*suttacentral\.net(?:\.au)?/?(?:[a-z]{2,3}/)?([a-z0-9.-]+)", hp)
        if not sm: continue
        slug = sm.group(1)
        if slug.startswith("pli-tv"): cands = [slug]
        else:
            fm = re.match(r"^([a-z]+?)(\d|$)", slug)
            if not fm or fm.group(1) not in families: continue
            cands = expand(slug)
        for muid in cands:
            if muid in seen: continue
            seen.add(muid)
            mine.setdefault(i + 1, []).append((muid, url))
items = re.findall(r"INSERT INTO collection_sutta_items \(collection_id, muid, position, group_id\) VALUES \(1, '([^']+)', (\d+), (\d+)\)", seed)
theirs = [m for m, p, g in items if g == "1"]
myg1 = [m for m, u in mine[1]]
print("mine_g1=%d theirs_g1=%d" % (len(myg1), len(theirs)))
print("only_mine=%s" % [m for m in myg1 if m not in theirs])
print("only_theirs=%s" % [m for m in theirs if m not in myg1])
for m in [m for m in myg1 if m not in theirs]:
    idx = [x[0] for x in mine[1]].index(m)
    print("context:", mine[1][max(0, idx - 2):idx + 3])
    print("url:", dict(mine[1])[m])
