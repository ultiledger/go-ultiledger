package consensus

import "github.com/deckarep/golang-set"

// Check whether the first set is the proper subset
// of the second subset
func IsProperSubset(a []string, b []string) bool {
	if len(a) > len(b) {
		return false
	}
	as := mapset.NewSet()
	for _, v := range a {
		as.Add(v)
	}
	bs := mapset.NewSet()
	for _, v := range b {
		bs.Add(v)
	}
	if as.IsProperSubset(bs) {
		return true
	}
	return false
}

// Check whether the first element is in the second slice
func IsElement(v string, ss []string) bool {
	if len(ss) == 0 {
		return false
	}
	for _, s := range ss {
		if v == s {
			return true
		}
	}
	return false
}
