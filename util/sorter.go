package util

import (
	"sort"
)

// Argsort utilities
type Slice struct {
	sort.Interface
	Idx  []int
	Desc bool
}

func (s Slice) Swap(i, j int) {
	s.Interface.Swap(i, j)
	s.Idx[i], s.Idx[j] = s.Idx[j], s.Idx[i]
}

func (s Slice) Less(i, j int) bool {
	if s.Desc {
		return s.Interface.Less(j, i)
	} else {
		return s.Interface.Less(i, j)
	}
}

func NewSlice(n sort.Interface, desc bool) *Slice {
	s := &Slice{
		Interface: n,
		Idx:       make([]int, n.Len()),
		Desc:      desc,
	}
	for i := range s.Idx {
		s.Idx[i] = i
	}
	return s
}

func NewIntSlice(desc bool, n ...int) *Slice         { return NewSlice(sort.IntSlice(n), desc) }
func NewFloat64Slice(desc bool, n ...float64) *Slice { return NewSlice(sort.Float64Slice(n), desc) }
func NewStringSlice(desc bool, n ...string) *Slice   { return NewSlice(sort.StringSlice(n), desc) }
