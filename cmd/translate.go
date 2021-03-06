package cmd

import (
	"fmt"
	"os"

	"github.com/evolbioinfo/goalign/align"
	"github.com/evolbioinfo/goalign/io"
	"github.com/spf13/cobra"
)

var translatePhase int
var translateOutput string
var translateGeneticCode string

// translateCmd represents the addid command
var translateCmd = &cobra.Command{
	Use:   "translate",
	Short: "Translates an input alignment in amino acids",
	Long: `Translates an input alignment in amino acids.

If the input alignment is not nucleotides, then returns an error.

It is possible to drop a given number of characters from the start 
of the alignment, by specifying the '--phase' option.

If given phase is -1, then it will translate in the 3 phases, 
from positions 0, 1 and 2. Sequence names will be added the suffix
_<phase>. At the end, 3x times more sequences will be present in the
file.

It is possible to specify alternative genetic code with --genetic-code 
(mitoi, mitov, or standard).

IUPAC codes are taken into account for the translation. If a codon containing 
IUPAC code is ambiguous for translation, then a X is added in place of the aminoacid.
`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var f *os.File
		var geneticcode int

		if f, err = openWriteFile(translateOutput); err != nil {
			io.LogError(err)
			return
		}
		defer closeWriteFile(f, translateOutput)

		switch translateGeneticCode {
		case "standard":
			geneticcode = align.GENETIC_CODE_STANDARD
		case "mitov":
			geneticcode = align.GENETIC_CODE_VETEBRATE_MITO
		case "mitoi":
			geneticcode = align.GENETIC_CODE_INVETEBRATE_MITO

		default:
			err = fmt.Errorf("Unknown genetic code : %s", translateGeneticCode)
			return
		}

		if unaligned {
			var seqs align.SeqBag

			if seqs, err = readsequences(infile); err != nil {
				io.LogError(err)
				return
			}
			if err = seqs.Translate(translatePhase, geneticcode); err != nil {
				io.LogError(err)
				return
			}
			writeSequences(seqs, f)
		} else {
			var aligns *align.AlignChannel
			var al align.Alignment

			if aligns, err = readalign(infile); err != nil {
				io.LogError(err)
				return
			}
			for al = range aligns.Achan {
				if err = al.Translate(translatePhase, geneticcode); err != nil {
					io.LogError(err)
					return
				}
				writeAlign(al, f)
			}

			if aligns.Err != nil {
				err = aligns.Err
				io.LogError(err)
			}
		}

		return
	},
}

func init() {
	RootCmd.AddCommand(translateCmd)
	translateCmd.PersistentFlags().StringVar(&translateGeneticCode, "genetic-code", "standard", "Genetic Code: standard, mitoi (invertebrate mitochondrial) or mitov (vertebrate mitochondrial)")
	translateCmd.PersistentFlags().StringVarP(&translateOutput, "output", "o", "stdout", "Output translated alignment file")
	translateCmd.PersistentFlags().IntVar(&translatePhase, "phase", 0, "Number of characters to drop from the start of the alignment (if -1: Translate in the 3 phases, from positions 0, 1, and 2)")
	translateCmd.PersistentFlags().BoolVar(&unaligned, "unaligned", false, "Considers sequences as unaligned and format fasta (phylip, nexus,... options are ignored)")
}
