package cmd

import (
	"os"

	"github.com/evolbioinfo/goalign/align"
	"github.com/evolbioinfo/goalign/io"
	"github.com/spf13/cobra"
)

// fastaCmd represents the fasta command
var fastaCmd = &cobra.Command{
	Use:   "fasta",
	Short: "Reformats an input alignment into Fasta",
	Long: `Reformats an alignment into Fasta. 
It may take a Phylip of Fasta input alignment.

If the input alignment contains several alignments, will take the first one only


Example of usage:

goalign reformat fasta -i align.phylip -p
goalign reformat fasta -i align.fasta

`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var f *os.File

		if f, err = openWriteFile(reformatOutput); err != nil {
			io.LogError(err)
			return
		}
		defer closeWriteFile(f, reformatOutput)

		if unaligned {
			var seqs align.SeqBag

			if seqs, err = readsequences(infile); err != nil {
				io.LogError(err)
				return
			}
			if reformatCleanNames {
				seqs.CleanNames(nil)
			}
			writeSequences(seqs, f)
		} else {
			var aligns *align.AlignChannel

			if aligns, err = readalign(infile); err != nil {
				io.LogError(err)
				return
			}

			a, _ := <-aligns.Achan
			if aligns.Err != nil {
				err = aligns.Err
				io.LogError(err)
				return
			}
			if reformatCleanNames {
				a.CleanNames(nil)
			}
			writeAlignFasta(a, f)
		}
		return
	},
}

func init() {
	reformatCmd.PersistentFlags().BoolVar(&unaligned, "unaligned", false, "Considers sequences as unaligned and format fasta (phylip, nexus,... options are ignored)")
	reformatCmd.AddCommand(fastaCmd)
}
