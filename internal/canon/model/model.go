package model

import "slices"

type Severity string

const (
	Info  Severity = "info"
	Warn  Severity = "warn"
	Error Severity = "error"
)

var severities = map[Severity]bool{Info: true, Warn: true, Error: true}

func (s Severity) Valid() bool { return severities[s] }

type CheckID string

const (
	CheckKeysetRootTranslation CheckID = "B001"
	CheckKeysetRootHTML        CheckID = "B002"
	CheckKeysetReference       CheckID = "B003"
	CheckDuplicateKeys         CheckID = "B004"
	CheckEmptyValue            CheckID = "B005"
	CheckNFCChanged            CheckID = "B006"
	CheckPrefixMismatch        CheckID = "B007"
	CheckEditionsCoverage      CheckID = "E101"
	CheckEditionsContent       CheckID = "E102"
	CheckEditionsStaleness     CheckID = "E103"
	CheckEditionsLastRun       CheckID = "E104"
	CheckDpdUnknownForms       CheckID = "D001"
	CheckDpdDegraded           CheckID = "D002"
)

var checks = []CheckID{
	CheckKeysetRootTranslation,
	CheckKeysetRootHTML,
	CheckKeysetReference,
	CheckDuplicateKeys,
	CheckEmptyValue,
	CheckNFCChanged,
	CheckPrefixMismatch,
	CheckEditionsCoverage,
	CheckEditionsContent,
	CheckEditionsStaleness,
	CheckEditionsLastRun,
	CheckDpdUnknownForms,
	CheckDpdDegraded,
}

func KnownChecks() []CheckID { return append([]CheckID(nil), checks...) }

func (c CheckID) Known() bool { return slices.Contains(checks, c) }

type NodeType string

const (
	NodeCollection NodeType = "collection"
	NodeDivision   NodeType = "division"
	NodeText       NodeType = "text"
	NodeSegment    NodeType = "segment"
	NodeEdition    NodeType = "edition"
	NodeSource     NodeType = "source"
	NodeHeadword   NodeType = "headword"
	NodeDpdRoot    NodeType = "dpd_root"
	NodeDpdForm    NodeType = "dpd_form"
)

type EdgeType string

const (
	EdgeChildOf     EdgeType = "CHILD_OF"
	EdgeTranslates  EdgeType = "TRANSLATES"
	EdgePartOf      EdgeType = "PART_OF"
	EdgeCoveredBy   EdgeType = "COVERED_BY"
	EdgeDerivedFrom EdgeType = "DERIVED_FROM"
	EdgeHasWord     EdgeType = "HAS_WORD"
)

type Stream string

const (
	StreamRoot        Stream = "root"
	StreamTranslation Stream = "translation"
	StreamHTML        Stream = "html"
	StreamReference   Stream = "reference"
)

type SegmentRecord struct {
	Stream Stream `json:"stream"`
	Muid   string `json:"muid"`
	SegKey string `json:"seg_key"`
	Text   string `json:"text"`
}

type NameRecord struct {
	Muid string `json:"muid"`
	Lang string `json:"lang"`
	Name string `json:"name"`
}

type HTMLSegment struct {
	Author string `json:"author"`
	Muid   string `json:"muid"`
	SegKey string `json:"seg_key"`
	Text   string `json:"text"`
}

type TexDoc struct {
	Path       string `json:"path"`
	Author     string `json:"author"`
	Collection string `json:"collection"`
	BuildDate  string `json:"build_date"`
	Text       string `json:"text"`
}

type DpdHeadword struct {
	Lemma1       string `json:"lemma_1"`
	Lemma2       string `json:"lemma_2"`
	Pos          string `json:"pos"`
	Grammar      string `json:"grammar"`
	Meaning1     string `json:"meaning_1"`
	MeaningLit   string `json:"meaning_lit"`
	Sanskrit     string `json:"sanskrit"`
	RootKey      string `json:"root_key"`
	RootSign     string `json:"root_sign"`
	FamilyRoot   string `json:"family_root"`
	Construction string `json:"construction"`
	CompoundType string `json:"compound_type"`
	Stem         string `json:"stem"`
}

type DpdRoot struct {
	Root         string `json:"root"`
	RootInComps  string `json:"root_in_comps"`
	RootHasVerb  string `json:"root_has_verb"`
	RootGroup    string `json:"root_group"`
	RootSign     string `json:"root_sign"`
	RootMeaning  string `json:"root_meaning"`
	SanskritRoot string `json:"sanskrit_root"`
}

type DpdForm struct {
	Form      string   `json:"form"`
	Headwords []string `json:"headwords"`
	Splits    []string `json:"splits"`
}

type Discrepancy struct {
	Check    CheckID  `json:"check"`
	Severity Severity `json:"severity"`
	Muid     string   `json:"muid,omitempty"`
	NodeID   int64    `json:"node_id,omitempty"`
	Detail   string   `json:"detail"`
}
