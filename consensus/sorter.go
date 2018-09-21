package consensus

import "strings"

// Custom sorter for ballot slice
type BallotSlice []*Ballot

// Return the length of underlying ballot slice
func (bs BallotSlice) Len() int {
	return len(bs)
}

// Sort ballots in descending order by comparing
// counter first then value
func (bs BallotSlice) Less(i, j int) bool {
	if bs[i].Counter > bs[j].Counter {
		return true
	} else if bs[i].Counter == bs[j].Counter {
		if strings.Compare(bs[i].Value, bs[j].Value) >= 0 {
			return true
		}
	}
	return false
}

func (bs BallotSlice) Swap(i, j int) {
	bs[i], bs[j] = bs[j], bs[i]
}
