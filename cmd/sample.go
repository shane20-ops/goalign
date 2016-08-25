package cmd

import (
	"github.com/fredericlemoine/goalign/io"
	"github.com/spf13/cobra"
	"math/rand"
	"time"
)

var sampleSeed int64
var sampleOutput string
var sampleNb int

// sampleCmd represents the sample command
var sampleCmd = &cobra.Command{
	Use:   "sample",
	Short: "Samples a subset of sequences from the input alignment",
	Long: `Samples a subset of sequences from the input alignment.

May take a Fasta or Phylip alignment in input.

As output, writes an alignment containing a sample of the sequences

`,
	Run: func(cmd *cobra.Command, args []string) {
		rand.Seed(sampleSeed)
		if sample, err := rootalign.Sample(sampleNb); err != nil {
			io.ExitWithMessage(err)
		} else {
			writeAlign(sample, sampleOutput)
		}
	},
}

func init() {
	RootCmd.AddCommand(sampleCmd)
	sampleCmd.PersistentFlags().IntVarP(&sampleNb, "nb-seq", "n", 1, "Number of sequences to sample from the alignment")
	sampleCmd.PersistentFlags().Int64VarP(&sampleSeed, "seed", "s", time.Now().UTC().UnixNano(), "Initial Random Seed")
	sampleCmd.PersistentFlags().StringVarP(&sampleOutput, "output", "o", "stdout", "Sampled alignment output file")
}