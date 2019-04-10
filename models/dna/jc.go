package dna

import (
	"github.com/evolbioinfo/goalign/align"
	"math"
)

type JCModel struct {
	numSites      float64 // Number of selected sites (no gaps)
	selectedSites []bool  // true for selected sites
	removegaps    bool    // If true, we will remove posision with >=1 gaps
}

func NewJCModel(removegaps bool) *JCModel {
	return &JCModel{
		0,
		nil,
		removegaps,
	}
}

/* computes JC69 distance between 2 sequences */
func (m *JCModel) Distance(seq1 []rune, seq2 []rune, weights []float64) (float64, error) {
	diff, total := countDiffs(seq1, seq2, m.selectedSites, weights)
	diff = diff / total
	dist := -3.0 / 4.0 * math.Log(1.0-4.0/3.0*diff)
	if dist > 0 {
		return dist, nil
	} else {
		return 0, nil
	}
}

func (m *JCModel) InitModel(al align.Alignment, weights []float64) (err error) {
	m.numSites, m.selectedSites = selectedSites(al, weights, m.removegaps)
	return
}

func (m *JCModel) Eigens() (val []float64, leftvector, rightvector [][]float64, err error) {
	val = []float64{
		0,
		-4. / 3.,
		-4. / 3.,
		-4. / 3.,
	}

	leftvector = [][]float64{
		[]float64{1. / 4., 1. / 4., 1. / 4., 1. / 4.},
		[]float64{-1. / 4., -1. / 4., -3. / 4., -1. / 4.},
		[]float64{-1. / 4., 3. / 4., -1. / 4., -1. / 4.},
		[]float64{3. / 4., -1. / 4., -1. / 4., -1. / 4.},
	}

	rightvector = [][]float64{
		[]float64{1., 0., 0., 1.},
		[]float64{1., 0., 1., 0.},
		[]float64{1., 1., 0., 0.},
		[]float64{1., -1., -1., -1.},
	}
	return
}