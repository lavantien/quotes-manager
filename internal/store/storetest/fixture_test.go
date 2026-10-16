package storetest

import "testing"

func TestCloneFixtureLoadsCorpus(t *testing.T) {
	s := CloneFixture(t)
	qs, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(qs) == 0 {
		t.Fatal("fixture has no quotes")
	}
	if _, err := s.CreateCategory("fixture-smoke"); err != nil {
		t.Fatalf("create category: %v", err)
	}
}
