package cmd

import (
	"github.com/spf13/cobra"
)

var shuffleOutput string

// shuffleCmd represents the shuffle command
var shuffleCmd = &cobra.Command{
	Use:   "shuffle",
	Short: "A set of commands to shuffle an alignment",
	Long: `A set of commands to shuffle an alignment.

It takes a Fasta of Phylip alignment in input.

It is possible to:
1 - Shuffle n sites vertically: It takes n sites of the input
    alignment and reassign the characters to different sequences;
2 - Shuffle sequence order in the alignment;
3 - Recombine n sequences together.

`,
}

func init() {
	RootCmd.AddCommand(shuffleCmd)

	shuffleCmd.PersistentFlags().StringVarP(&shuffleOutput, "output", "o", "stdout", "Shuffled alignment output file")
}
