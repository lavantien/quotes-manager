import re
import sys

dump_path = "dumps/ariyasavaka-curated-list.md"
seed_path = "database/seed.sql"

raw = open(dump_path, "rb").read()
crlf = raw.count(b"\r\n")
text = raw.decode("utf-8")

heading_re = re.compile(r"^\*\*([IVXLCDM]+)\.\s*(.+?)\*\*", re.M)
link_re = re.compile(r"\[([^\]]*)\]\(([^()\s]+)\)")
slug_re = re.compile(r"^/?(?:[a-z]{2,3}/)?([a-z0-9.-]+)")
families = {"mn", "dn", "an", "sn", "ud", "iti", "snp", "dhp", "khp", "vv", "ja", "thag", "thig"}
num_re = re.compile(r"^([a-z]+?)(\d+(?:\.\d+)?)$")

def norm(s):
    parts = s.split(".")
    out = []
    for p in parts:
        t = p.lstrip("0") or "0"
        out.append(t)
    return ".".join(out)

def expand(slug):
    m = re.match(r"^([a-z]+?)(\d+(?:\.\d+)?)-(\d+(?:\.\d+)?)$", slug)
    if not m:
        return [slug], False
    fam, a, b = m.group(1), norm(m.group(2)), norm(m.group(3))
    parts_a = a.split(".")
    parts_b = b.split(".")
    if len(parts_a) == 1 and len(parts_b) == 1:
        return [f"{fam}{n}" for n in range(int(a), int(b) + 1)], True
    ca = parts_a[0] if len(parts_a) > 1 else parts_b[0]
    cb = parts_b[0] if len(parts_b) > 1 else parts_a[0]
    if ca != cb:
        return [slug], True
    na = parts_a[-1]
    nb = parts_b[-1]
    return [f"{fam}{ca}.{n}" for n in range(int(na), int(nb) + 1)], True

groups = []
for hm in heading_re.finditer(text):
    groups.append((hm.group(1), hm.group(2).rstrip("."), hm.end()))

if not groups:
    sys.exit("no headings found")

seen = set()
sizes = []
first = None
last = None
members = {}
for i, (roman, name, start) in enumerate(groups):
    end = groups[i + 1][2] if i + 1 < len(groups) else len(text)
    body = text[start:end]
    count = 0
    for lm in link_re.finditer(body):
        url = lm.group(2)
        host_path = re.sub(r"^[a-z]+://", "", url)
        if not (host_path.startswith("suttacentral.net") or host_path.startswith("www.suttacentral.net") or host_path.startswith("dhammatalks.org")):
            continue
        m = slug_re.match("/" + host_path.split("/", 1)[1] if "/" in host_path else "")
        if "suttacentral" in host_path:
            path = "/" + host_path.split("/", 1)[1]
        elif "dhammatalks" in host_path:
            dm = re.search(r"StNp(\d+(?:\.\d+)?)(?:_(\d+))?\.html", host_path)
            if not dm:
                continue
            muid = "snp" + norm(dm.group(1)) + ("." + norm(dm.group(2)) if dm.group(2) else "")
            candidates = [muid]
        if "suttacentral" in host_path:
            sm = re.match(r"^/([a-z]{2,3}/)?([a-z0-9.-]+)", path)
            if not sm:
                continue
            slug = sm.group(2)
            if slug.startswith("pli-tv"):
                candidates = [slug]
            else:
                fm = re.match(r"^([a-z]+?)(\d)", slug)
                if not fm or fm.group(1) not in families:
                    continue
                candidates, _ = expand(slug)
        for muid in candidates:
            if muid in seen:
                continue
            seen.add(muid)
            count += 1
            if first is None:
                first = muid
            last = muid
            members.setdefault(i, []).append(muid)
    sizes.append(count)

print("crlf_count=%d" % crlf)
print("headings=%d romans=%s" % (len(groups), ",".join(g[0] for g in groups)))
print("sizes=%s" % sizes)
print("total=%d first=%s last=%s" % (len(seen), first, last))

seed = open(seed_path, encoding="utf-8").read()
seed_suttas = re.findall(r"INSERT INTO suttas \(muid, display, link\) VALUES \('([^']+)'", seed)
seed_groups = re.findall(r"INSERT INTO collection_groups \(id, collection_id, name, position\) VALUES \((\d+), 1, '([^']*)', (\d+)\)", seed)
seed_items = re.findall(r"INSERT INTO collection_sutta_items \(collection_id, muid, position, group_id\) VALUES \(1, '([^']+)', (\d+), (\d+)\)", seed)
print("seed: suttas=%d groups=%d items=%d" % (len(seed_suttas), len(seed_groups), len(seed_items)))
if set(seen) == set(seed_suttas):
    dump_order = [members[i][j] for i in range(len(groups)) for j in range(len(members.get(i, [])))]
    seed_order = [m for m, _, _ in seed_items]
    print("seed: item_muids_match_dump_order=%s" % (dump_order == seed_order))
else:
    print("seed: item_muids_match_dump_order=set-mismatch")
per_group = {}
for muid, pos, gid in seed_items:
    per_group.setdefault(int(gid), []).append(muid)
seed_sizes = [len(per_group.get(g + 1, [])) for g in range(len(seed_groups))]
print("seed_sizes=%s" % seed_sizes)
print("sizes_match=%s total_match=%s" % (seed_sizes == sizes, sorted(seen) == sorted(seed_suttas)))
