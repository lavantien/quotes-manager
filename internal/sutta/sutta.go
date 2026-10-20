package sutta

type Sutta struct {
	Muid    string
	Display string
	Link    string
}

type Group struct {
	Name   string
	Suttas []Sutta
}

type Table struct {
	Name   string
	Groups []Group
}

type File struct {
	Name    string
	Content string
}
