package dna

import (
	"fmt"

	"github.com/evolbioinfo/goalign/align"
)

const (
	GAP_COUNT_NONE     = 0
	GAP_COUNT_INTERNAL = 1
	GAP_COUNT_ALL      = 2
)

// Like pdist, but without
// Normalization by the number
// of sites
type RawDistModel struct {
	numSites      float64 // Number of selected sites (no gaps)
	selectedSites []bool  // true for selected sites
	removegaps    bool    // If true, we will remove posision with >=1 gaps
	// If 0, will not count as 1 mutation '-' to 'A"
	// If 1, will count as 1 mutation '-' to 'A"
	// If 2, will count as 1 mutation '-' to 'A", but only the internal
	// Default 0
	countgapmut   int
	sequenceCodes [][]uint8 // Sequences converted into int codes
}

func NewRawDistModel(removegaps bool) *RawDistModel {
	return &RawDistModel{
		numSites:      0,
		selectedSites: nil,
		removegaps:    removegaps,
		countgapmut:   0,
		sequenceCodes: nil,
	}
}

func (m *RawDistModel) SetCountGapMutations(countgapmut int) (err error) {
	if countgapmut < 0 || countgapmut > 2 {
		err = fmt.Errorf("Gap count mode not available : %d", countgapmut)
	} else {
		m.countgapmut = countgapmut
	}
	return
}

// computes the number of differences  between 2 sequences
// These differences include gaps vs. nt
func (m *RawDistModel) Distance(seq1 []uint8, seq2 []uint8, weights []float64) (diff float64, err error) {
	switch m.countgapmut {
	case GAP_COUNT_ALL:
		diff, _ = countDiffsWithGaps(seq1, seq2, m.selectedSites, weights)
	case GAP_COUNT_INTERNAL:
		diff, _ = countDiffsWithInternalGaps(seq1, seq2, m.selectedSites, weights)
	default:
		diff, _ = countDiffs(seq1, seq2, m.selectedSites, weights)
	}
	return
}

func (m *RawDistModel) InitModel(al align.Alignment, weights []float64, gamma bool, alpha float64) (err error) {
	m.numSites, m.selectedSites = selectedSites(al, weights, m.removegaps)
	m.sequenceCodes, err = alignmentToCodes(al)
	return
}

// Sequence returns the ith sequence of the alignment
// encoded in int
func (m *RawDistModel) Sequence(i int) (seq []uint8, err error) {
	if i < 0 || i >= len(m.sequenceCodes) {
		err = fmt.Errorf("This sequence does not exist: %d", i)
		return
	}
	seq = m.sequenceCodes[i]
	return
}
