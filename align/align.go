package align

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"strings"
	"unicode"

	"github.com/armon/go-radix"
	"github.com/evolbioinfo/goalign/io"
)

// Alignment represents a set of aligned sequences (multiple Sequence Alignment)
type Alignment interface {
	SeqBag
	AddGaps(rate, lenprop float64)
	Append(Alignment) error // Appends alignment sequences to this alignment
	AvgAllelesPerSite() float64
	BuildBootstrap() Alignment // Bootstrap alignment
	CharStatsSite(site int) (map[rune]int, error)
	Clone() (Alignment, error)
	CodonAlign(ntseqs SeqBag) (codonAl *align, err error)
	// Remove identical patterns/sites and return number of occurence
	// of each pattern (order of patterns/sites may have changed)
	Compress() []int
	// concatenates the given alignment with this alignment
	Concat(Alignment) error
	// Computes the majority consensus of the given alignemnt
	// To do so, it takes the majority character at each alignment site
	// if excludeGaps is true, then gaps are not taken into account for majority computation
	Consensus(excludeGaps bool) *align
	// Compares all sequences to the first one and counts all differences per sequence
	//
	// - alldiffs: The set of all differences that have been seen at least once
	// - diffs   : The number of occurences of each difference, for each sequence
	//             Sequences are ordered as the original alignment. Differences are
	//             written as REFNEW, ex: diffs["AC"]=12 .
	CountDifferences() (alldiffs []string, diffs []map[string]int)
	// Compares all sequences to the first one and replace identical characters with .
	DiffWithFirst()
	Entropy(site int, removegaps bool) (float64, error) // Entropy of the given site
	// Positions of potential frameshifts
	// if startinggapsasincomplete is true, then considers gaps as the beginning
	// as incomplete sequence, then take the right phase
	Frameshifts(startingGapsAsIncomplete bool) []struct{ Start, End int }
	// Positions of potential stop in frame
	// if startinggapsasincomplete is true, then considers gaps as the beginning
	// as incomplete sequence, then take the right phase
	Stops(startingGapsAsIncomplete bool, geneticode int) (stops []int, err error)
	Length() int                  // Length of the alignment
	Mask(start, length int) error // Masks given positions
	MaxCharStats(excludeGaps bool) ([]rune, []int)
	Mutate(rate float64)  // Adds uniform substitutions in the alignment (~sequencing errors)
	NbVariableSites() int // Nb of variable sites
	// Number of Gaps in each sequence that are unique in their alignment site
	NumGapsUniquePerSequence(countProfile *CountProfile) (numuniques []int, numnew []int, numboth []int, err error)
	// returns the number of characters in each sequence that are unique in their alignment site (gaps or others)
	// It does not take into account 'N' and '-' as unique mutations
	NumMutationsUniquePerSequence(profile *CountProfile) (numuniques []int, numnew []int, nummuts []int, err error)
	Pssm(log bool, pseudocount float64, normalization int) (pssm map[rune][]float64, err error) // Normalization: PSSM_NORM_NONE, PSSM_NORM_UNIF, PSSM_NORM_DATA
	Rarefy(nb int, counts map[string]int) (Alignment, error)                                    // Take a new rarefied sample taking into accounts weights
	RandSubAlign(length int) (Alignment, error)                                                 // Extract a random subalignment with given length from this alignment
	Recombine(rate float64, lenprop float64)
	// converts coordinates on the given sequence to coordinates on the alignment
	RefCoordinates(name string, refstart, refend int) (alistart, aliend int, err error)
	RemoveGapSeqs(cutoff float64)                               // Removes sequences having >= cutoff gaps
	RemoveGapSites(cutoff float64, ends bool) (first, last int) // Removes sites having >= cutoff gaps, returns the number of consecutive removed sites at start and end of alignment
	// Replaces match characters (.) by their corresponding characters on the first sequence
	ReplaceMatchChars()
	Sample(nb int) (Alignment, error) // generate a sub sample of the sequences
	ShuffleSites(rate float64, roguerate float64, randroguefirst bool) []string
	SimulateRogue(prop float64, proplen float64) ([]string, []string) // add "rogue" sequences
	SiteConservation(position int) (int, error)                       // If the site is conserved:
	Split(part *PartitionSet) ([]Alignment, error)                    //Splits the alignment given the paritions in argument
	SubAlign(start, length int) (Alignment, error)                    // Extract a subalignment from this alignment
	Swap(rate float64)
	TrimSequences(trimsize int, fromStart bool) error
}

type align struct {
	seqbag
	length int // Length of alignment
}

// AlignChannel is used for iterating over alignments
type AlignChannel struct {
	Achan chan Alignment
	Err   error
}

// NewAlign initializes a new alignment
func NewAlign(alphabet int) *align {
	switch alphabet {
	case AMINOACIDS, NUCLEOTIDS, UNKNOWN:
		// OK
	case BOTH:
		alphabet = NUCLEOTIDS
	default:
		io.ExitWithMessage(errors.New("Unexpected sequence alphabet type"))
	}
	return &align{
		seqbag{
			make(map[string]*seq),
			make([]*seq, 0, 100),
			false,
			alphabet},
		-1,
	}
}

// AlphabetFromString converts the alphabet name to its code
// If the alphabet name is not known, returns align.UNKNOWN
func AlphabetFromString(alphabet string) int {
	switch strings.ToLower(alphabet) {
	case "dna", "rna", "nucleotide":
		return NUCLEOTIDS
	case "protein":
		return AMINOACIDS
	default:
		return UNKNOWN
	}
}

// AddSequence Adds a sequence to this alignment
func (a *align) AddSequence(name string, sequence string, comment string) error {
	err := a.AddSequenceChar(name, []rune(sequence), comment)
	return err
}

// AddSequenceChar adds a sequence from its rune representation.
// If a.ignoreidentical is true, then it won't add the sequence if
// a sequence with the same name AND same sequence
// already exists in the alignment
func (a *align) AddSequenceChar(name string, sequence []rune, comment string) error {
	s, ok := a.seqmap[name]
	idx := 0
	tmpname := name

	// If the sequence name already exists with the same sequence
	// and ignoreidentical is true, then we ignore this sequence
	if ok && a.ignoreidentical && s.SameSequence(sequence) {
		log.Print(fmt.Sprintf("Warning: sequence \"%s\" already exists in alignment with the same sequence, ignoring", name))
		return nil
	}

	/* If the sequence name already exists, we add a 4 digit index at the end and print a warning on stderr */
	for ok {
		idx++
		log.Print(fmt.Sprintf("Warning: sequence \"%s\" already exists in alignment, renamed in \"%s_%04d\"", tmpname, name, idx))
		tmpname = fmt.Sprintf("%s_%04d", name, idx)
		_, ok = a.seqmap[tmpname]
		/*return errors.New("Sequence " + name + " already exists in alignment")*/
	}

	if a.length != -1 && a.length != len(sequence) {
		return errors.New("Sequence " + tmpname + " does not have same length as other sequences")
	}
	a.length = len(sequence)
	seq := NewSequence(tmpname, sequence, comment)
	a.seqmap[tmpname] = seq
	a.seqs = append(a.seqs, seq)
	return nil
}

// Clear removes all the sequences from the alignment
func (a *align) Clear() {
	a.seqbag.Clear()
	a.length = -1
}

// Length returns the current length of the alignment
func (a *align) Length() int {
	return a.length
}

// Shuffles vertically rate sites of the alignment
// randomly
// rate must be >=0 and <=1
// Then, take roguerate proportion of the taxa, and will shuffle rate sites among the
// remaining intact sites
// randroguefirst: If true, then with a given seed, rogues will always be the same with all alignments
// having sequences in the same order. It may not be the case if false, especially when alignemnts
// have different lengths.
// Output: List of tax names that are more shuffled than others (length=roguerate*nbsequences)
func (a *align) ShuffleSites(rate float64, roguerate float64, randroguefirst bool) []string {
	var sitepermutation, taxpermutation []int

	if rate < 0 || rate > 1 {
		io.ExitWithMessage(errors.New("Shuffle site rate must be >=0 and <=1"))
	}
	if roguerate < 0 || roguerate > 1 {
		io.ExitWithMessage(errors.New("Shuffle rogue rate must be >=0 and <=1"))
	}

	nbSitesToShuffle := int(rate * float64(a.Length()))
	nbRogueSitesToShuffle := int(rate * (1.0 - rate) * (float64(a.Length())))
	nbRogueSeqToShuffle := int(roguerate * float64(a.NbSequences()))
	if randroguefirst {
		taxpermutation = rand.Perm(a.NbSequences())
		sitepermutation = rand.Perm(a.Length())
	} else {
		sitepermutation = rand.Perm(a.Length())
		taxpermutation = rand.Perm(a.NbSequences())
	}

	rogues := make([]string, nbRogueSeqToShuffle)

	if (nbRogueSitesToShuffle + nbSitesToShuffle) > a.Length() {
		io.ExitWithMessage(fmt.Errorf("Too many sites to shuffle (%d+%d>%d)",
			nbRogueSitesToShuffle, nbSitesToShuffle, a.Length()))
	}

	var temp rune
	for i := 0; i < nbSitesToShuffle; i++ {
		site := sitepermutation[i]
		var n int = a.NbSequences()
		for n > 1 {
			r := rand.Intn(n)
			n--
			temp = a.seqs[n].sequence[site]
			a.seqs[n].sequence[site] = a.seqs[r].sequence[site]
			a.seqs[r].sequence[site] = temp
		}
	}
	// We shuffle more sites for "rogue" taxa
	for i := 0; i < nbRogueSitesToShuffle; i++ {
		site := sitepermutation[i+nbSitesToShuffle]
		for r := 0; r < nbRogueSeqToShuffle; r++ {
			j := rand.Intn(r + 1)
			seq1 := a.seqs[taxpermutation[r]]
			seq2 := a.seqs[taxpermutation[j]]
			seq1.sequence[site], seq2.sequence[site] = seq2.sequence[site], seq1.sequence[site]
			rogues[r] = seq1.name
		}
	}
	return rogues
}

// Removes positions constituted of [cutoff*100%,100%] Gaps
// Exception fo a cutoff of 0: does not remove positions with 0% gaps
// Cutoff must be between 0 and 1, otherwise set to 0.
// 0 means that positions with > 0 gaps will be removed
// other cutoffs : ]0,1] mean that positions with >= cutoff gaps will be removed
//
// If ends is true: then only removes consecutive positions that match the cutoff
// from start or from end of alignment.
// Example with a cutoff of 0.3 and ends and with the given proportion of gaps:
// 0.4 0.5 0.1 0.5 0.6 0.1 0.8 will remove positions 0,1 and 6
//
// Returns the number of consecutive removed sites at start and end of alignment
func (a *align) RemoveGapSites(cutoff float64, ends bool) (first, last int) {
	var nbgaps int
	if cutoff < 0 || cutoff > 1 {
		cutoff = 0
	}

	toremove := make([]int, 0, 10)
	// To remove only gap positions at start and ends positions
	firstcontinuous := -1
	lastcontinuous := a.Length()

	for site := 0; site < a.Length(); site++ {
		nbgaps = 0
		for seq := 0; seq < a.NbSequences(); seq++ {
			if a.seqs[seq].sequence[site] == GAP {
				nbgaps++
			}
		}
		if (cutoff > 0.0 && float64(nbgaps) >= cutoff*float64(a.NbSequences())) || (cutoff == 0 && nbgaps > 0) {
			toremove = append(toremove, site)
			if site == firstcontinuous+1 {
				firstcontinuous++
			}
		}
	}

	first = firstcontinuous + 1

	/* Now we remove gap positions, starting at the end */
	sort.Ints(toremove)
	nbremoved := 0
	for i := (len(toremove) - 1); i >= 0; i-- {
		if toremove[i] == lastcontinuous-1 {
			lastcontinuous--
		}

		if !ends || lastcontinuous == toremove[i] || toremove[i] <= firstcontinuous {
			nbremoved++
			for seq := 0; seq < a.NbSequences(); seq++ {
				a.seqs[seq].sequence = append(a.seqs[seq].sequence[:toremove[i]], a.seqs[seq].sequence[toremove[i]+1:]...)
			}
		}
	}
	last = a.Length() - lastcontinuous
	a.length -= nbremoved

	return first, last
}

// RefCoordinates converts coordinates on the given sequence to coordinates on the alignment.
// Coordinates on the given sequence corresponds to the sequence without gaps. Output coordinates
// on the alignent consider gaps.
//
// It returns an error if the sequence does not exist or if the coordinates are outside the ref
// sequence (<0 or > sequence length without gaps)
// Parameters:
//    - name: The name of the sequence to take as reference
//    - refstart: The start coordinate to convert (on the ref sequence, 0-based)
//    - reflen: The length of the ref sequence to consider from refstart
func (a *align) RefCoordinates(name string, refstart, reflen int) (alistart, alilen int, err error) {
	var exists bool
	var seq []rune
	var tmpi int
	var site rune
	var ngaps int

	if seq, exists = a.GetSequenceChar(name); !exists {
		err = fmt.Errorf("Sequence %s does not exist in the alignment", name)
		return
	}
	if refstart < 0 {
		err = fmt.Errorf("Start on reference sequence must be > 0 : %d", refstart)
		return
	}
	if reflen <= 0 {
		err = fmt.Errorf("Reference length must be > 0 : %d", reflen)
		return
	}

	alistart = 0
	alilen = 0
	//look for start
	tmpi = -1
	for _, site = range seq {
		if site != GAP {
			tmpi++
		} else {
			ngaps++
		}

		if tmpi < refstart {
			alistart++
		} else {
			alilen++
			if tmpi >= refstart+reflen-1 {
				break
			}
		}
	}

	if refstart+reflen > len(seq)-ngaps {
		err = fmt.Errorf("Start + Length (%d + %d) on reference sequence falls outside the sequence", refstart, reflen)
	}

	return
}

// Removes sequences constituted of [cutoff*100%,100%] Gaps
// Exception fo a cutoff of 0: does not remove sequences with 0% gaps
// Cutoff must be between 0 and 1, otherwise set to 0.
// 0 means that sequences with > 0 gaps will be removed
// other cutoffs : ]0,1] mean that sequences with >= cutoff gaps will be removed
func (a *align) RemoveGapSeqs(cutoff float64) {
	var nbgaps int
	if cutoff < 0 || cutoff > 1 {
		cutoff = 0
	}
	oldseqs := a.seqs
	length := a.Length()
	a.Clear()
	for _, seq := range oldseqs {
		nbgaps = 0
		for site := 0; site < length; site++ {
			if seq.sequence[site] == GAP {
				nbgaps++
			}
		}
		if !((cutoff > 0.0 && float64(nbgaps) >= cutoff*float64(length)) || (cutoff == 0 && nbgaps > 0)) {
			a.AddSequenceChar(seq.name, seq.sequence, seq.comment)
		}
	}
}

// Swaps a rate of the sequences together
// takes rate/2 seqs and swap a part of them with the other
// rate/2 seqs at a random position
// if rate < 0 : does nothing
// if rate > 1 : does nothing
func (a *align) Swap(rate float64) {
	var nb_to_shuffle, nb_sites int
	var pos int
	var tmpchar rune
	var seq1, seq2 *seq

	if rate < 0 || rate > 1 {
		return
	}
	nb_sites = a.Length()
	nb_to_shuffle = (int)(rate * float64(a.NbSequences()))

	permutation := rand.Perm(a.NbSequences())

	for i := 0; i < int(nb_to_shuffle/2); i++ {
		// We take a random position in the sequences and swap both
		pos = rand.Intn(nb_sites)
		seq1 = a.seqs[permutation[i]]
		seq2 = a.seqs[permutation[i+(int)(nb_to_shuffle/2)]]
		for pos < nb_sites {
			tmpchar = seq1.sequence[pos]
			seq1.sequence[pos] = seq2.sequence[pos]
			seq2.sequence[pos] = tmpchar
			pos++
		}
	}
}

// Replace an old string in sequences by a new string
// It may be a regexp
//
// - If it changes the length of the sequences, then returns an error and the returned alignment
// is changed anyway
// - If the regex is malformed, returns an error
func (a *align) Replace(old, new string, regex bool) (err error) {

	if err = a.seqbag.Replace(old, new, regex); err != nil {
		return
	}
	// Verify that sequences still have same length
	a.IterateChar(func(name string, s []rune) bool {
		if len(s) != a.Length() {
			err = fmt.Errorf("replace should not change the length of aligned sequences")
			return true
		}
		return false
	})

	return
}

// Replaces match characters (.) by their corresponding characters on the first sequence
//
// If the correspongind character in the first sequence is also a ".", then leaves it unchanged.
func (a *align) ReplaceMatchChars() {
	if a.NbSequences() <= 1 {
		return
	}
	ref := a.seqs[0]
	for seq := 1; seq < a.NbSequences(); seq++ {
		for site := 0; site < a.Length(); site++ {
			if ref.sequence[site] != POINT && a.seqs[seq].sequence[site] == POINT {
				a.seqs[seq].sequence[site] = ref.sequence[site]
			}
		}
	}
}

// Translates the alignment, and update the length of
// the alignment
func (a *align) Translate(phase int, geneticcode int) (err error) {
	err = a.seqbag.Translate(phase, geneticcode)
	if len(a.seqs) > 0 {
		a.length = len(a.seqs[0].sequence)
	} else {
		a.length = -1
	}
	return
}

// Recombines a rate of the sequences to another sequences
// takes rate/2 seqs and copy/paste a portion of them to the other
// rate/2 seqs at a random position
// if rate < 0 : does nothing
// if rate > 1 : does nothing
// prop must be <= 0.5 because it will recombine x% of seqs based on other x% of seqs
func (a *align) Recombine(prop float64, lenprop float64) {
	var seq1, seq2 *seq

	if prop < 0 || prop > 0.5 {
		return
	}
	if lenprop < 0 || lenprop > 1 {
		return
	}

	nb := int(prop * float64(a.NbSequences()))
	lentorecomb := int(lenprop * float64(a.Length()))
	permutation := rand.Perm(a.NbSequences())

	// We take a random position in the sequences between min and max
	for i := 0; i < nb; i++ {
		pos := rand.Intn(a.Length() - lentorecomb + 1)
		seq1 = a.seqs[permutation[i]]
		seq2 = a.seqs[permutation[i+nb]]
		for j := pos; j < pos+lentorecomb; j++ {
			seq1.sequence[j] = seq2.sequence[j]
		}
	}
}

// Add prop*100% gaps to lenprop*100% of the sequences
// if prop < 0 || lenprop<0 : does nothing
// if prop > 1 || lenprop>1 : does nothing
func (a *align) AddGaps(lenprop float64, prop float64) {
	if prop < 0 || prop > 1 {
		return
	}
	if lenprop < 0 || lenprop > 1 {
		return
	}

	nb := int(prop * float64(a.NbSequences()))
	nbgaps := int(lenprop * float64(a.Length()))
	permseqs := rand.Perm(a.NbSequences())

	// We take a random position in the sequences between min and max
	for i := 0; i < nb; i++ {
		permsites := rand.Perm(a.Length())
		seq := a.seqs[permseqs[i]]
		for j := 0; j < nbgaps; j++ {
			seq.sequence[permsites[j]] = GAP
		}
	}
}

func (a *align) Append(al Alignment) (err error) {
	al.IterateAll(func(name string, sequence []rune, comment string) bool {
		err = a.AddSequenceChar(name, sequence, comment)
		return err != nil

	})
	return
}

func (a *align) append(al *align) (err error) {
	for _, s := range al.seqs {
		if err = a.AddSequenceChar(s.name, s.sequence, s.comment); err != nil {
			return
		}
	}
	return
}

// Add substitutions uniformly to the alignment
// if rate < 0 : does nothing
// if rate > 1 : rate=1
// It does not apply to gaps or other special characters
func (a *align) Mutate(rate float64) {
	if rate <= 0 {
		return
	}
	if rate > 1 {
		rate = 1
	}
	r := 0.0
	newchar := 0
	leng := a.Length()
	nb := a.NbSequences()
	// We take a random position in the sequences between min and max
	for i := 0; i < nb; i++ {
		seq := a.seqs[i]
		for j := 0; j < leng; j++ {
			r = rand.Float64()
			// We mutate only if rand is <= rate && character is not a gap
			// or a special character.
			// It takes a random nucleotide or amino acid uniformly
			if r <= rate && seq.sequence[j] != GAP && seq.sequence[j] != POINT && seq.sequence[j] != OTHER {
				if a.Alphabet() == AMINOACIDS {
					newchar = rand.Intn(len(stdaminoacid))
					seq.sequence[j] = stdaminoacid[newchar]
				} else {
					newchar = rand.Intn(len(stdnucleotides))
					seq.sequence[j] = stdnucleotides[newchar]
				}
			}
		}
	}
}

// Simulate rogue taxa in the alignment:
// take the proportion prop of sequences as rogue taxa => R
// For each t in R
//   * We shuffle the alignment sites of t
// Output: List of rogue sequence names, and List of intact sequence names
func (a *align) SimulateRogue(prop float64, proplen float64) ([]string, []string) {
	var seq *seq

	if prop < 0 || prop > 1.0 {
		return nil, nil
	}

	if proplen < 0 || proplen > 1.0 {
		return nil, nil
	}

	if proplen == 0 {
		prop = 0.0
	}

	nb := int(prop * float64(a.NbSequences()))
	permutation := rand.Perm(a.NbSequences())
	seqlist := make([]string, nb)
	intactlist := make([]string, a.NbSequences()-nb)
	len := int(proplen * float64(a.Length()))
	// For each chosen rogue sequence
	for r := 0; r < nb; r++ {
		seq = a.seqs[permutation[r]]
		seqlist[r] = seq.name
		sitesToShuffle := rand.Perm(a.Length())[0:len]
		// we Shuffle some sequence sites
		for i, _ := range sitesToShuffle {
			j := rand.Intn(i + 1)
			seq.sequence[sitesToShuffle[i]], seq.sequence[sitesToShuffle[j]] = seq.sequence[sitesToShuffle[j]], seq.sequence[sitesToShuffle[i]]
		}
	}
	for nr := nb; nr < a.NbSequences(); nr++ {
		seq = a.seqs[permutation[nr]]
		intactlist[nr-nb] = seq.name
	}
	return seqlist, intactlist
}

// Trims alignment sequences.
// If fromStart, then trims from the start, else trims from the end
// If trimsize >= sequence or trimsize < 0 lengths, then throw an error
func (a *align) TrimSequences(trimsize int, fromStart bool) error {
	if trimsize < 0 {
		return errors.New("Trim size must not be < 0")
	}
	if trimsize >= a.Length() {
		return errors.New("Trim size must be < alignment length (" + fmt.Sprintf("%d", a.Length()) + ")")
	}
	for _, seq := range a.seqs {
		if fromStart {
			seq.sequence = seq.sequence[trimsize:len(seq.sequence)]
		} else {
			seq.sequence = seq.sequence[0 : len(seq.sequence)-trimsize]
		}
	}
	a.length = a.length - trimsize
	return nil
}

// Samples randomly a subset of the sequences
// And returns this new alignment
// If nb < 1 or nb > nbsequences returns nil and an error
func (a *align) Sample(nb int) (al Alignment, err error) {
	var sampleSeqBag *seqbag
	var ali *align

	if sampleSeqBag, err = a.sampleSeqBag(nb); err != nil {
		return
	}

	if ali, err = seqBagToAlignment(sampleSeqBag); err != nil {
		return
	}

	al = ali

	return
}

/*
Each sequence in the alignment has an associated number of occurence. The sum s of the counts
represents the number of sequences in the underlying initial dataset.

The goal is to downsample (rarefy) the initial dataset, by sampling n sequences
from s (n<s), and taking the alignment corresponding to this new sample, i.e by
taking only unique (different) sequences from it.

Parameters are:
* nb: the number of sequences to sample from the underlying full dataset (different
from the number of sequences in the output alignment)
* counts: counts associated to each sequence (if the count of a sequence is missing, it
is considered as 0, if the count of an unkown sequence is present, it will return an error).
 Sum of counts of all sequences must be > n.
*/
func (a *align) Rarefy(nb int, counts map[string]int) (al Alignment, err error) {
	var rarefySeqBag *seqbag
	var ali *align

	if rarefySeqBag, err = a.rarefySeqBag(nb, counts); err != nil {
		return
	}

	if ali, err = seqBagToAlignment(rarefySeqBag); err != nil {
		return
	}

	al = ali

	return
}

// This function builds a bootstrap alignment
// and returns it with "indices", an array containing
// the index (in the original alignment) of all bootstrap sites.
func (a *align) BuildBootstrap() (boot Alignment) {
	n := a.Length()
	boot = NewAlign(a.alphabet)
	indices := make([]int, n)
	var buf []rune

	for i := 0; i < n; i++ {
		indices[i] = rand.Intn(n)
	}

	for _, seq := range a.seqs {
		buf = make([]rune, n)
		for i, indice := range indices {
			buf[i] = seq.sequence[indice]
		}
		boot.AddSequenceChar(seq.name, buf, seq.Comment())
	}
	return
}

// Returns the distribution of characters at a given site
// if the site index is outside alignment, returns an error
func (a *align) CharStatsSite(site int) (outmap map[rune]int, err error) {
	outmap = make(map[rune]int)

	if site < 0 || site >= a.Length() {
		err = errors.New("Cannot compute site char statistics: Site index is outside alignment")
	} else {

		for _, s := range a.seqs {
			outmap[unicode.ToUpper(s.sequence[site])]++
		}
	}
	return outmap, err
}

// Mask masks sites of the alignment, going from start,
// with a given length.
// - For aa sequences: It masks with X
// - For nt sequences: It masks with N
func (a *align) Mask(start, length int) (err error) {
	if start < 0 {
		err = errors.New("Mask: Start position cannot be < 0")
		return
	}
	if start > a.Length() {
		err = errors.New("Mask: Start position cannot be > align length")
		return
	}

	rep := '.'
	if a.Alphabet() == AMINOACIDS {
		rep = ALL_AMINO
	} else if a.Alphabet() == NUCLEOTIDS {
		rep = ALL_NUCLE
	} else {
		err = errors.New("Mask: Cannot mask alignment, wrong alphabet")
	}

	for _, seq := range a.seqs {
		for i := start; i < (start+length) && i < a.Length(); i++ {
			seq.sequence[i] = rep
		}
	}
	return
}

// Returns the Character with the most occurences
// for each site of the alignment
func (a *align) MaxCharStats(excludeGaps bool) (out []rune, occur []int) {
	out = make([]rune, a.Length())
	occur = make([]int, a.Length())
	for site := 0; site < a.Length(); site++ {
		mapstats := make(map[rune]int)
		max := 0
		for _, seq := range a.seqs {
			mapstats[unicode.ToUpper(seq.sequence[site])]++
		}

		out[site] = GAP
		occur[site] = len(a.seqs)
		for k, v := range mapstats {
			// If we exclude gaps and it is a gap: we do nothing
			// Otherwise, if v > max, we update max occurence char
			if !(excludeGaps && k == GAP) && v > max {
				out[site] = k
				occur[site] = v
				max = v
			}
		}
	}

	return out, occur
}

// RandomAlignment generates a random alignment with a given alphabet
// length and number of sequences. Each character is randomly choosen
// in a uniform distribution.
func RandomAlignment(alphabet, length, nbseq int) (al Alignment, err error) {
	var seq []rune
	al = NewAlign(alphabet)
	for i := 0; i < nbseq; i++ {
		name := fmt.Sprintf("Seq%04d", i)
		if seq, err = RandomSequence(alphabet, length); err != nil {
			return
		}
		al.AddSequenceChar(name, seq, "")
	}
	return
}

func (a *align) Clone() (c Alignment, err error) {
	c = NewAlign(a.Alphabet())
	c.IgnoreIdentical(a.ignoreidentical)
	a.IterateAll(func(name string, sequence []rune, comment string) bool {
		newseq := make([]rune, 0, len(sequence))
		newseq = append(newseq, sequence...)
		err = c.AddSequenceChar(name, newseq, comment)
		return err != nil
	})
	return
}

func (a *align) AvgAllelesPerSite() float64 {
	nballeles := 0
	nbsites := 0
	for site := 0; site < a.Length(); site++ {
		alleles := make(map[rune]bool)
		onlygap := true
		for seq := 0; seq < a.NbSequences(); seq++ {
			s := a.seqs[seq].sequence[site]
			if s != GAP && s != POINT && s != OTHER {
				alleles[s] = true
				onlygap = false
			}
		}
		if !onlygap {
			nbsites++
		}
		nballeles += len(alleles)
	}
	return float64(nballeles) / float64(nbsites)
}

// Entropy of the given site. If the site number is < 0 or > length -> returns an error
// if removegaps is true, do not take into account gap characters
func (a *align) Entropy(site int, removegaps bool) (float64, error) {
	if site < 0 || site > a.Length() {
		return 1.0, errors.New("Site position is outside alignment")
	}

	// Number of occurences of each different aa/nt
	occur := make(map[rune]int)
	total := 0
	entropy := 0.0
	for seq := 0; seq < a.NbSequences(); seq++ {
		s := a.seqs[seq].sequence[site]
		if s != OTHER && s != POINT && (!removegaps || s != GAP) {
			nb, ok := occur[s]
			if !ok {
				occur[s] = 1
			} else {
				occur[s] = nb + 1
			}
			total++
		}
	}

	for _, v := range occur {
		proba := float64(v) / float64(total)
		entropy -= proba * math.Log(proba)
	}

	if total == 0 {
		return math.NaN(), nil
	}
	return entropy, nil
}

// First sequence of the alignment is considered as the reference orf (in phase)
// It return for each sequence the coordinates of the longest dephased part
func (a *align) Frameshifts(startingGapsAsIncomplete bool) (fs []struct{ Start, End int }) {
	ref := a.seqs[0]
	fs = make([]struct{ Start, End int }, a.NbSequences())
	for s := 1; s < a.NbSequences(); s++ {
		fs[s].Start = 0
		fs[s].End = 0
		seq := a.seqs[s]
		phase := 0 // in frame
		start := 0 // Start of dephasing
		pos := 0
		started := false
		for i := 0; i < a.Length(); i++ {
			// Insertion in seq
			if ref.sequence[i] == '-' {
				phase++
				phase = (phase % 3)
			}
			// Deletion in seq
			if seq.sequence[i] == '-' {
				phase--
				if phase < 0 {
					phase = 2
				}
			} else if !started && startingGapsAsIncomplete && phase != 0 {
				phase--
				if phase < 0 {
					phase = 2
				}
			} else {
				started = true
				pos++
			}

			// If we go back in the good phase (or we are at the end of the sequence:
			// we add a frameshift if it is longer than the previous one
			if (phase == 0 || i == a.Length()-1) && pos-start > 1 && pos-start > fs[s].End-fs[s].Start {
				fs[s].Start = start
				fs[s].End = pos
			}

			if phase == 0 {
				start = pos
			}
		}
	}
	return
}

// Position of the first encountered STOP in frame
func (a *align) Stops(startingGapsAsIncomplete bool, geneticcode int) (stops []int, err error) {
	var code map[string]rune

	if code, err = geneticCode(geneticcode); err != nil {
		return
	}

	stops = make([]int, a.NbSequences())
	codon := make([]rune, 3)
	ref := a.seqs[0]
	phase := 0
	started := false
	for s := 1; s < a.NbSequences(); s++ {
		seq := a.seqs[s]
		stops[s] = -1
		pos := 0      // position on sequence (without -)
		codonpos := 0 // nb nt in current codon
		for i := 0; i < a.Length()-2; i++ {
			if ref.sequence[i] == '-' {
				phase++
				phase = (phase % 3)
			}
			// Deletion in seq
			if seq.sequence[i] == '-' {
				phase--
				if phase < 0 {
					phase = 2
				}
			} else if !started && startingGapsAsIncomplete && phase != 0 {
				phase--
				if phase < 0 {
					phase = 2
				}
			} else {
				started = true
			}

			// Deletion in seq
			if seq.sequence[i] != '-' && (!startingGapsAsIncomplete || started) {
				codon[codonpos] = seq.sequence[i]
				codonpos++
				pos++
			}

			if codonpos == 3 {
				codonstr := strings.Replace(strings.ToUpper(string(codon)), "U", "T", -1)
				aa, found := code[codonstr]
				if !found {
					aa = 'X'
				}
				if aa == '*' {
					stops[s] = pos
					break
				}
				codonpos = 0
			}
		}
	}
	return
}

/* Computes a position-specific scoring matrix (PSSM)matrix
(see https://en.wikipedia.org/wiki/Position_weight_matrix)
This matrix may be in log2 scale or not (log argument)
A pseudo count may be added to values (to avoid log2(0))) with pseudocount argument
values may be normalized: normalization arg:
   PSSM_NORM_NONE = 0 => No normalization
   PSSM_NORM_FREQ = 1 => Normalization by frequency in the site
   PSSM_NORM_DATA = 2 => Normalization by frequency in the site and divided by aa/nt frequency in data
   PSSM_NORM_UNIF = 3 => Normalization by frequency in the site and divided by uniform frequency (1/4 or 1/20)
   PSSM_NORM_LOGO = 4 => Normalization like "Logo"
*/
func (a *align) Pssm(log bool, pseudocount float64, normalization int) (pssm map[rune][]float64, err error) {
	// Number of occurences of each different aa/nt
	pssm = make(map[rune][]float64)
	var alphabet []rune
	var normfactors map[rune]float64
	/* Entropy at each position */
	var entropy []float64
	alphabet = a.AlphabetCharacters()
	for _, c := range alphabet {
		if _, ok := pssm[c]; !ok {
			pssm[c] = make([]float64, a.Length())
		}
	}

	/* We compute normalization factors (takes into account pseudo counts) */
	normfactors = make(map[rune]float64)
	switch normalization {
	case PSSM_NORM_NONE:
		for _, c := range alphabet {
			normfactors[c] = 1.0
		}
	case PSSM_NORM_UNIF:
		for _, c := range alphabet {
			normfactors[c] = 1.0 / (float64(a.NbSequences()) + (float64(len(pssm)) * pseudocount)) / (1.0 / float64(len(alphabet)))
		}
	case PSSM_NORM_FREQ:
		for _, c := range alphabet {
			normfactors[c] = 1.0 / (float64(a.NbSequences()) + (float64(len(pssm)) * pseudocount))
		}
	case PSSM_NORM_LOGO:
		for _, c := range alphabet {
			normfactors[c] = 1.0 / float64(a.NbSequences())
		}
	case PSSM_NORM_DATA:
		stats := a.CharStats()
		total := 0.0
		for _, c := range alphabet {
			if s, ok := stats[c]; !ok {
				err = errors.New(fmt.Sprintf("No charchacter %c in alignment statistics", c))
				return
			} else {
				total += float64(s)
			}
		}
		for _, c := range alphabet {
			s, _ := stats[c]
			normfactors[c] = 1.0 / (float64(a.NbSequences()) + (float64(len(pssm)) * pseudocount)) / (float64(s) / total)
		}
	default:
		err = errors.New("Unknown normalization option")
		return
	}

	/* We count nt/aa occurences at each site */
	for site := 0; site < a.Length(); site++ {
		for seq := 0; seq < a.NbSequences(); seq++ {
			s := a.seqs[seq].sequence[site]
			s = unicode.ToUpper(s)
			if _, ok := normfactors[s]; ok {
				if _, ok := pssm[s]; ok {
					pssm[s][site] += 1.0
				}
			}
		}
	}

	/* We add pseudo counts */
	if pseudocount > 0 {
		for _, v := range pssm {
			for i := range v {
				v[i] += pseudocount
			}
		}
	}

	/* Initialize entropy if NORM_LOGO*/
	entropy = make([]float64, a.Length())
	/* Applying normalization factors */
	for k, v := range pssm {
		for i, _ := range v {
			v[i] = v[i] * normfactors[k]
			if normalization == PSSM_NORM_LOGO {
				entropy[i] += -v[i] * math.Log(v[i]) / math.Log(2)
			}
		}
	}

	/* We compute the logo */
	if normalization == PSSM_NORM_LOGO {
		for _, v := range pssm {
			for i, _ := range v {
				v[i] = v[i] * (math.Log(float64(len(alphabet)))/math.Log(2) - entropy[i])
			}
		}
	} else {
		/* Applying log2 transform */
		if log {
			for _, v := range pssm {
				for i, _ := range v {
					v[i] = math.Log(v[i]) / math.Log(2)
				}
			}
		}
	}

	return
}

// Extract a subalignment from this alignment
func (a *align) SubAlign(start, length int) (subalign Alignment, err error) {
	if start < 0 || start > a.Length() {
		err = fmt.Errorf("Start is outside the alignment")
		return
	}
	if length < 0 {
		err = fmt.Errorf("Length is negative")
		return
	}
	if start+length < 0 || start+length > a.Length() {
		err = fmt.Errorf("Start+Length is outside the alignment")
		return
	}
	subalign = NewAlign(a.alphabet)
	for i := 0; i < a.NbSequences(); i++ {
		seq := a.seqs[i]
		subalign.AddSequenceChar(seq.name, seq.SequenceChar()[start:start+length], seq.Comment())
	}
	return
}

// Extract a subalignment with given length and a random start position from this alignment
func (a *align) RandSubAlign(length int) (Alignment, error) {
	if length > a.Length() {
		return nil, errors.New("sub alignment is larger than original alignment ")
	}
	if length <= 0 {
		return nil, errors.New("sub alignment cannot have 0 or negative length")
	}

	subalign := NewAlign(a.alphabet)
	start := rand.Intn(a.Length() - length + 1)
	for i := 0; i < a.NbSequences(); i++ {
		seq := a.seqs[i]
		subalign.AddSequenceChar(seq.name, seq.SequenceChar()[start:start+length], seq.Comment())
	}
	return subalign, nil
}

/*
Remove identical patterns/sites and return number of occurence
 of each pattern (order of patterns/sites may have changed)
*/
func (a *align) Compress() (weights []int) {
	var count interface{}
	var ok bool
	r := radix.New()
	npat := 0
	// We add new patterns if not already insterted in the radix tree
	for site := 0; site < a.Length(); site++ {
		pattern := make([]rune, a.NbSequences())
		for seq := 0; seq < a.NbSequences(); seq++ {
			pattern[seq] = a.seqs[seq].sequence[site]
		}
		patstring := string(pattern)
		if count, ok = r.Get(patstring); !ok {
			npat++
			count = &struct{ count int }{0}
		}
		count.(*struct{ count int }).count++
		r.Insert(patstring, count)
	}
	// Init weights
	weights = make([]int, npat)
	// We add the patterns
	npat = 0
	r.Walk(func(pattern string, count interface{}) bool {
		weights[npat] = count.(*struct{ count int }).count
		for seq, c := range pattern {
			a.seqs[seq].sequence[npat] = c
		}
		npat++
		return false
	})
	// We remove what remains of the sequences after al patterns
	for seq := 0; seq < a.NbSequences(); seq++ {
		a.seqs[seq].sequence = a.seqs[seq].sequence[:npat]
	}
	a.length = npat
	return
}

/*
Concatenates both alignments. It appends the given alignment to this alignment.
If a sequence is present in this alignment and not in c, then it adds a full gap sequence.
If a sequence is present in c alignment and not in this, then it appends the new sequence
to a full gap sequence.
Returns an error if the sequences do not have the same alphabet.
*/
func (a *align) Concat(c Alignment) (err error) {
	if a.Alphabet() != c.Alphabet() {
		return errors.New("Alignments do not have the same alphabet")
	}
	a.IterateAll(func(name string, sequence []rune, comment string) bool {
		_, ok := c.GetSequenceChar(name)
		if !ok {
			// This sequence is present in a but not in c
			// So we append full gap sequence to a
			err = a.appendToSequence(name, []rune(strings.Repeat(string(GAP), c.Length())))
		}
		return err != nil
	})
	if err != nil {
		return err
	}
	c.IterateAll(func(name string, sequence []rune, comment string) bool {
		_, ok := a.GetSequenceChar(name)
		if !ok {
			// This sequence is present in c but not in a
			// So we add it to a, with gaps only
			err = a.AddSequence(name, strings.Repeat(string(GAP), a.Length()), comment)
		}
		// Then we append the c sequence to a
		err = a.appendToSequence(name, sequence)
		return err != nil
	})
	if err != nil {
		return err
	}

	leng := -1
	a.IterateChar(func(name string, sequence []rune) bool {
		if leng == -1 {
			leng = len(sequence)
		} else {
			if leng != len(sequence) {
				err = errors.New("Sequences of the new alignment do not have the same length")
			}
		}
		return err != nil
	})
	a.length = leng

	return err
}

// Computes the majority consensus of the given alignemnt
// To do so, it takes the majority character at each alignment site
//
// if excludeGaps is true, then gaps are not taken into account for
// majority computation
func (a *align) Consensus(excludeGaps bool) (cons *align) {
	var consseq []rune
	consseq, _ = a.MaxCharStats(excludeGaps)

	cons = NewAlign(a.Alphabet())

	cons.AddSequenceChar("consensus", consseq, "")

	return
}

// Compares all sequences to the first one and replaces identical characters with .
func (a *align) DiffWithFirst() {
	var first []rune
	var i, l int
	if a.NbSequences() < 2 {
		return
	}

	i = 0
	a.IterateChar(func(name string, other []rune) bool {
		if i == 0 {
			first = other
		} else {
			for l = 0; l < len(first); l++ {
				if first[l] == other[l] {
					other[l] = '.'
				}
			}
		}
		i++
		return false
	})
}

// Compares all sequences to the first one and counts all differences per sequence
//
// - alldiffs: The set of all differences that have been seen at least once
// - diffs   : The number of occurences of each difference, for each sequence
//             Sequences are ordered as the original alignment. Differences are
//             written as REFNEW, ex: diffs["AC"]=12.
func (a *align) CountDifferences() (alldiffs []string, diffs []map[string]int) {
	var alldiffsmap map[string]bool
	var diffmap map[string]int
	var first []rune
	var key string
	var ok bool
	var i, l, count int

	alldiffs = make([]string, 0)
	diffs = make([]map[string]int, a.NbSequences()-1)
	if a.NbSequences() < 2 {
		return
	}

	alldiffsmap = make(map[string]bool, 0)
	i = 0
	a.IterateChar(func(name string, other []rune) bool {
		if i == 0 {
			first = other
		} else {
			diffmap = make(map[string]int)
			for l = 0; l < len(first); l++ {
				if first[l] != other[l] {
					key = fmt.Sprintf("%c%c", first[l], other[l])
					count = diffmap[key]
					diffmap[key] = count + 1
					if _, ok = alldiffsmap[key]; !ok {
						alldiffs = append(alldiffs, key)
						alldiffsmap[key] = true
					}
				}
			}
			diffs[i-1] = diffmap
		}
		i++
		return false
	})

	return
}

/*
 Returns the number of variable sites in the alignment.
It does not take into account gaps and other charactes like "."
*/
func (a *align) NbVariableSites() int {
	nbinfo := 0
	for site := 0; site < a.Length(); site++ {
		charmap := make(map[rune]bool)
		variable := false
		for _, seq := range a.seqs {
			if seq.sequence[site] != GAP && seq.sequence[site] != POINT && seq.sequence[site] != OTHER {
				charmap[seq.sequence[site]] = true
			}
			if len(charmap) > 1 {
				variable = true
				break
			}
		}
		if variable {
			nbinfo++
		}
	}
	return nbinfo
}

// NumGapsUniquePerSequence returns the number of Gaps in the sequence that are unique in their alignment site
// This function counts, for each sequence of the given alignment, the number of :
// - gaps that are unique to the sequence compared to the others of the alignment
// - gaps that are new compared to the profile (not seen in the profile) : numnew
// - gaps that are new compared to the profile and found only once in the given alignment: numboth
// If the profile is nil, then does not compute numnewmuts neither nummutsboth (0 filled slices)
func (a *align) NumGapsUniquePerSequence(countProfile *CountProfile) (numuniques []int, numnew []int, numboth []int, err error) {
	numuniques = make([]int, a.NbSequences())
	numnew = make([]int, a.NbSequences())
	numboth = make([]int, a.NbSequences())

	uniqueIndex := -1
	nbGapsColumn := 0

	// Check that profile has the right length
	if countProfile != nil {
		if !countProfile.CheckLength(a.Length()) {
			err = fmt.Errorf("Profile does not have same length than alignment")
			return
		}
	}

	var c int
	for i := 0; i < a.Length(); i++ {
		uniqueIndex = -1
		nbGapsColumn = 0

		for j, s := range a.seqs {
			r := s.sequence[i]
			if r == GAP {
				nbGapsColumn++
				uniqueIndex = j
				if countProfile != nil {
					c, _ = countProfile.Count(r, i)
					if c == 0 {
						numnew[j]++
					}
				} else if nbGapsColumn > 1 {
					break
				}
			}
		}

		if nbGapsColumn == 1 {
			numuniques[uniqueIndex]++
			if countProfile != nil {
				if c, _ = countProfile.Count(GAP, i); c == 0 {
					numboth[uniqueIndex]++
				}
			}
		}
	}
	return
}

// NumMutationsUniquePerSequence returns the number of characters in each sequence that are unique in their alignment site.
// It does not take into account 'N' and '-' as unique mutations
// This function counts, for each sequence of the given alignment, the number of :
// - mutations that are unique to the sequence compared to the others of the alignment
// - mutations that are new compared to the profile (not seen in the profile) : numnew
// - mutations that are new compared to the profile and found only once in the given alignment: numboth
// If the profile is nil, then does not compute numnewmuts neither nummutsboth (0 filled slices)
func (a *align) NumMutationsUniquePerSequence(countProfile *CountProfile) (numuniques []int, numnew []int, numboth []int, err error) {
	numuniques = make([]int, a.NbSequences())
	numnew = make([]int, a.NbSequences())
	numboth = make([]int, a.NbSequences())

	all := '.'
	if a.Alphabet() == AMINOACIDS {
		all = ALL_AMINO
	} else if a.Alphabet() == NUCLEOTIDS {
		all = ALL_NUCLE
	}

	// Check that profile has the right length
	if countProfile != nil {
		if !countProfile.CheckLength(a.Length()) {
			err = fmt.Errorf("Profile does not have same length than alignment")
			return
		}
	}

	var c int
	for i := 0; i < a.Length(); i++ {
		occurences := make([]int, 130)
		indices := make([]int, 130)

		for j, s := range a.seqs {
			r := s.sequence[i]
			occurences[int(r)]++
			indices[int(r)] = j
			if countProfile != nil && r != all && r != GAP {
				if c, _ = countProfile.Count(r, i); c == 0 {
					numnew[j]++
				}
			}
		}

		for c, num := range occurences {
			if num == 1 && rune(c) != all && rune(c) != GAP {
				ind := indices[c]
				numuniques[ind]++
				if countProfile != nil {
					if c, _ = countProfile.Count(rune(c), i); c == 0 {
						numboth[ind]++
					}
				}
			}
		}
	}
	return
}

// Aligns given nt sequences (ntseqs) using a corresponding aa alignment (a).
//
// If a is not amino acid, then returns an error.
// If ntseqs is not nucleotides then returns an error.
//
// Warning: It does not check that the amino acid sequence is a good
// translation of the nucleotide sequence, but just adds gaps to the
// nucleotide sequence where needed.
//
// Once gaps are added, if the nucleotide alignment length does not match
// the protein alignment length * 3, returns an error.
func (a *align) CodonAlign(ntseqs SeqBag) (rtAl *align, err error) {
	var buffer bytes.Buffer

	if a.Alphabet() != AMINOACIDS {
		return nil, errors.New("Wrong alphabet, cannot reverse translate nucleotides")
	}

	if ntseqs.Alphabet() != NUCLEOTIDS {
		return nil, errors.New("Wrong nucleotidic alignment alphabet, cannot reverse translate")
	}

	rtAl = NewAlign(ntseqs.Alphabet())
	// outputting aligned codons
	a.IterateAll(func(name string, sequence []rune, comment string) bool {
		buffer.Reset()
		ntseq, ok := ntseqs.GetSequenceChar(name)
		if !ok {
			err = fmt.Errorf("Sequence %s is not present in the nucleotidic sequence, cannot reverse translate", name)
			return true
		}

		ntseqindex := 0
		for i := 0; i < len(sequence); i++ {
			if sequence[i] == '-' {
				buffer.WriteString("---")
			} else {
				if ntseqindex+3 > len(ntseq) {
					err = fmt.Errorf("Nucleotidic sequence %s is shorter than its aa counterpart", name)
					return true
				}
				buffer.WriteString(string(ntseq[ntseqindex : ntseqindex+3]))
				ntseqindex += 3
			}
		}
		if ntseqindex < len(ntseq) {
			// At most 2 remaining nucleotides that could not be part of the last codon
			if len(ntseq)-ntseqindex <= 2 {
				log.Print(fmt.Sprintf("%s: Dropping %d additional nucleotides", name, len(ntseq)-ntseqindex))
			} else {
				// A problem with the sequences
				err = fmt.Errorf("Nucleotidic sequence %s is longer than its aa counterpart (%d = more than 2 nucleotides remaining)", name, len(ntseq)-ntseqindex)
				return true
			}
		}
		rtAl.AddSequence(name, buffer.String(), comment)
		return false
	})
	return
}

// Compute conservation status of a given site of the alignment
//
// If position is outside the alignment, it returns an error
//
// Possible values are:
//
// - align.POSITION_IDENTICAL
// - align.POSITION_CONSERVED
// - align.POSITION_SEMI_CONSERVED
// - align.POSITION_NOT_CONSERVED
func (a *align) SiteConservation(position int) (conservation int, err error) {
	conservation = POSITION_NOT_CONSERVED

	if position < 0 || position >= a.Length() {
		err = errors.New("Site conservation: Position is not in sequence length range")
		return
	}

	tmpstronggroups := make([]int, len(strongGroups))
	tmpweakgroups := make([]int, len(weakGroups))
	same := true
	prevchar := ';'
	a.IterateChar(func(name string, sequence []rune) bool {
		if a.Alphabet() == AMINOACIDS {
			for i, g := range strongGroups {
				for _, aa := range g {
					if aa == unicode.ToUpper(sequence[position]) {
						tmpstronggroups[i]++
					}
				}
			}
			for i, g := range weakGroups {
				for _, aa := range g {
					if aa == unicode.ToUpper(sequence[position]) {
						tmpweakgroups[i]++
					}
				}
			}
		}
		if (prevchar != ';' && sequence[position] != prevchar) || sequence[position] == GAP {
			same = false
		}
		prevchar = sequence[position]
		return false
	})

	if same {
		conservation = POSITION_IDENTICAL
	} else {
		for _, nb := range tmpstronggroups {
			if nb == a.NbSequences() {
				conservation = POSITION_CONSERVED
			}
		}
		if conservation != POSITION_CONSERVED {
			for _, nb := range tmpweakgroups {
				if nb == a.NbSequences() {
					conservation = POSITION_SEMI_CONSERVED
				}
			}
		}
	}

	return
}

// Use the partition to generate one alignment per partition.
//
// If the partitionset has one partition or less, then returns an error
func (a *align) Split(part *PartitionSet) (als []Alignment, err error) {
	if part.NPartitions() <= 1 {
		err = fmt.Errorf("The given partitionset contains less than 2 partitions")
		return
	}

	if part.AliLength() != a.Length() {
		err = fmt.Errorf("The given partitionset has a different alignment length")
		return
	}

	als = make([]Alignment, part.NPartitions())
	alsimpl := make([]*align, part.NPartitions())
	for pi := 0; pi < part.NPartitions(); pi++ {
		alsimpl[pi] = NewAlign(a.Alphabet())
		als[pi] = alsimpl[pi]
		firstpos := true
		for pos := 0; pos < part.AliLength(); pos++ {
			if part.Partition(pos) == pi {
				for si := 0; si < a.NbSequences(); si++ {
					seq := a.seqs[si]
					if firstpos {
						alsimpl[pi].AddSequenceChar(seq.Name(), []rune{seq.CharAt(pos)}, seq.Comment())
					} else {
						alsimpl[pi].seqs[si].sequence = append(alsimpl[pi].seqs[si].sequence, seq.sequence[pos])
					}
				}
				if firstpos {
					alsimpl[pi].length = 1
				} else {
					alsimpl[pi].length++
				}
				firstpos = false
			}
		}
	}
	return
}

func seqBagToAlignment(sb *seqbag) (al *align, err error) {
	al = NewAlign(sb.Alphabet())

	// We just check that sequence lengths are all equal
	al.length = -1
	sb.IterateChar(func(name string, s []rune) bool {
		l := len(s)
		if al.length != -1 && al.length != l {
			err = fmt.Errorf("Sequence %s does not have same length as other sequences", name)
			return true
		}
		al.length = l
		return false
	})

	//If ok, we transfer the structures to the new alignment (same reference!)
	al.seqbag = *sb

	return
}
