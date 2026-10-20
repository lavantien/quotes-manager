//go:build windows

package main

import (
	"slices"
	"testing"
)

func TestTreeOrderWalksTreeDepthFirst(t *testing.T) {
	children := map[uint32][]uint32{1: {2, 3}, 2: {4}, 3: {5, 6}}
	got := treeOrder(children, 1)
	if !slices.Equal(got, []uint32{1, 2, 4, 3, 5, 6}) {
		t.Fatalf("treeOrder = %v", got)
	}
}

func TestTreeOrderTerminatesOnParentageCycles(t *testing.T) {
	children := map[uint32][]uint32{1: {2}, 2: {1}, 5: {5}, 7: {8, 8}}
	got := treeOrder(children, 1)
	if !slices.Equal(got, []uint32{1, 2}) {
		t.Fatalf("treeOrder on cycle = %v, want [1 2]", got)
	}
	self := treeOrder(children, 5)
	if !slices.Equal(self, []uint32{5}) {
		t.Fatalf("treeOrder on self-parent = %v, want [5]", self)
	}
}

func TestTreeOrderIncludesChildlessRoot(t *testing.T) {
	if got := treeOrder(map[uint32][]uint32{1: {2}}, 9); !slices.Equal(got, []uint32{9}) {
		t.Fatalf("treeOrder on childless root = %v, want [9]", got)
	}
}
