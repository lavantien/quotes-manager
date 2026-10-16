import io
import sys

ROOT = r"C:\Users\lavantien\dev\github\quotes-manager"
SRC = ROOT + r"\internal\server\server_test.go"

raw = io.open(SRC, newline="", encoding="utf-8").read()
eol = "\r\n" if "\r\n" in raw else "\n"
lines = raw.replace("\r\n", "\n").split("\n")
if lines and lines[-1] == "":
    lines = lines[:-1]

SEGS = {
    "fakestore": [
        (18, 472, "type fakeStore struct {"),
        (1329, 1367, "type failingStore struct{}"),
        (1518, 1553, "type failList struct{ *fakeStore }"),
    ],
    "collections": [
        (524, 533, "func TestCollectionViewShowsBlockCount"),
        (731, 869, "func fakeWithCollection"),
        (941, 960, "func TestRailRightListsCollections"),
        (1204, 1297, "func TestRenameCollectionHandler"),
        (1316, 1329, "func TestRootBlockShowsCollectionChips"),
        (1466, 1510, "func TestCollectionReorderBadJSON"),
        (1692, 1701, "func TestRenameCollectionEmptyName"),
    ],
    "categories": [
        (869, 941, "func fakeWithCategory"),
        (960, 1057, "func TestCreateCategory"),
        (1057, 1179, "func TestQuoteChips"),
        (1179, 1204, "func TestIndexSidebarAndChips"),
        (1297, 1316, "func TestRootPaneCategoryFilter"),
        (1682, 1692, "func TestRenameCategoryEmptyName"),
    ],
    "keep": [
        (473, 524, "func newServer"),
        (533, 731, "func TestCreateAddsQuote"),
        (1368, 1416, "func TestHandlerStoreErrors"),
        (1416, 1466, "func TestNewForm"),
        (1510, 1518, "func TestIndexUnknownCategoryEmptyRoot"),
        (1553, 1681, "func assert500"),
    ],
}

for name, ranges in SEGS.items():
    for a, b, want in ranges:
        got = next(l for l in lines[a - 1 : b - 1] if l.strip())
        if not got.startswith(want):
            sys.exit("segment %s (%d,%d): first line %r does not start with %r" % (name, a, b, got, want))


def seg(a, b):
    return "\n".join(lines[a - 1 : b - 1]).strip("\n")


def header(std, internal):
    out = ["package server_test", "", "import ("]
    out += ['\t"%s"' % s for s in std]
    if internal:
        out.append("")
        out += ['\t"%s"' % s for s in internal]
    out.append(")")
    return "\n".join(out)


FILES = {
    r"\internal\server\server_fakestore_test.go": (
        ["errors", "fmt", "sort", "strings"],
        ["github.com/lavantien/quotes-manager/internal/quote", "github.com/lavantien/quotes-manager/internal/store"],
        "fakestore",
    ),
    r"\internal\server\server_collections_test.go": (
        ["fmt", "net/http", "strings", "testing"],
        [],
        "collections",
    ),
    r"\internal\server\server_categories_test.go": (
        ["fmt", "net/http", "strings", "testing"],
        [],
        "categories",
    ),
    r"\internal\server\server_test.go": (
        ["fmt", "io", "net/http", "net/http/httptest", "strings", "testing"],
        [
            "github.com/lavantien/quotes-manager/internal/quote",
            "github.com/lavantien/quotes-manager/internal/server",
            "github.com/lavantien/quotes-manager/internal/store",
        ],
        "keep",
    ),
}


def skip_sig(l):
    s = l.strip()
    return not s or s == ")" or l.startswith(("package ", "import (", '\t"'))


old_sig = sorted(l for l in lines if not skip_sig(l))
new_sig = []
for rel, (std, internal, key) in FILES.items():
    text = header(std, internal) + "\n\n" + "\n\n".join(seg(a, b) for a, b, _ in SEGS[key]) + "\n"
    if eol == "\r\n":
        text = text.replace("\n", "\r\n")
    io.open(ROOT + rel, "w", newline="", encoding="utf-8").write(text)
    new_sig += [l for l in text.replace("\r\n", "\n").split("\n") if not skip_sig(l)]

if old_sig != sorted(new_sig):
    from collections import Counter

    diff = Counter(new_sig) - Counter(old_sig)
    diff -= Counter(old_sig) - Counter(new_sig)
    sys.exit("line multiset mismatch: %r" % (list(diff.items())[:10],))
print("split ok: line multisets identical, eol=%r" % eol)
