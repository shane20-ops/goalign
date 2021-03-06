package cmd

import (
	"fmt"
	"os"

	"github.com/evolbioinfo/goalign/align"
	"github.com/evolbioinfo/goalign/io"
	"github.com/spf13/cobra"
)

var cleanEnds bool

// cleansitesCmd represents the cleansites command
var cleansitesCmd = &cobra.Command{
	Use:   "sites",
	Short: "Removes sites with gaps",
	Long: `Removes sites constituted of gaps

Removes sites constitued of >= cutoff gap sites.

Exception for a cutoff of 0: removes sites constitued of > 0 gap sites.

Examples:
- With a cutoff of 0.5: a site with 5 gaps over 10 sequences will be removed;
- With a cutoff of 0.5: a site with 4 gaps over 10 sequences will not be removed;
- With a cutoff of 0.0 a site with 1 gap over 10 sequences will be removed.

If cutoff is <0 or >1, it will be considered as 0, which means that every site with at least 1 gap
will be removed.`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		var aligns *align.AlignChannel
		var nbstart, nbend int
		var f *os.File

		if aligns, err = readalign(infile); err != nil {
			io.LogError(err)
			return
		}
		if f, err = openWriteFile(cleanOutput); err != nil {
			io.LogError(err)
			return
		}
		defer closeWriteFile(f, cleanOutput)

		i := 0
		for al := range aligns.Achan {
			beforelength := al.Length()
			nbstart, nbend = al.RemoveGapSites(cleanCutoff, cleanEnds)
			afterlength := al.Length()
			writeAlign(al, f)
			if !cleanQuiet {
				io.PrintSimpleMessage(fmt.Sprintf("Alignment (%d) length before cleaning=%d", i, beforelength))
				io.PrintSimpleMessage(fmt.Sprintf("Alignment (%d) length after cleaning=%d", i, afterlength))
				io.PrintSimpleMessage(fmt.Sprintf("Alignment (%d) number of gaps=%d", i, beforelength-afterlength))
				io.PrintSimpleMessage(fmt.Sprintf("Alignment (%d) number of start gaps=%d", i, nbstart))
				io.PrintSimpleMessage(fmt.Sprintf("Alignment (%d) number of end gaps=%d", i, nbend))
			}
		}

		if aligns.Err != nil {
			err = aligns.Err
			io.LogError(err)
		}
		return
	},
}

func init() {
	cleansitesCmd.PersistentFlags().BoolVar(&cleanEnds, "ends", false, "If true, then only remove consecutive gap positions from alignment start and end")
	cleanCmd.AddCommand(cleansitesCmd)
}
