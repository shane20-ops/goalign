package cmd

import (
	"fmt"
	"math"

	"github.com/spf13/cobra"
)

var entropyAverage bool

// entropyCmd represents the entropy command
var entropyCmd = &cobra.Command{
	Use:   "entropy",
	Short: "Computes entropy of a given alignment",
	Long: `Computes entropy of a given alignment.

Example: 
goalign compute entropy -i alignment.fa
goalign compute entropy -i alignment.phy -p

It is possible to compute the average entropy:
goalign compute entropy -i alignment.phy -p -a

Which will print one average entropy per alignment in the input file:
Alignment \t AvgEntropy


Otherwise, it will print one entropy per alignment site, in a tab separated form:
Alignment \t Site \t Entropy

the computation does not take into account the following characters:

-> '*'
-> '-'

If a site is made fully of '-' or '*', then its entropy will be "NaN", and it will not
be taken into account in the average.

`,
	Run: func(cmd *cobra.Command, args []string) {
		nb := 0
		if entropyAverage {
			fmt.Println("Alignment\tAvgEntropy")
		} else {
			fmt.Println("Alignment\tSite\tEntropy")
		}
		avg := 0.0
		for align := range rootaligns {
			total := 0
			for i := 0; i < align.Length(); i++ {
				if e, err := align.Entropy(i); err != nil {
					panic(err)
				} else {
					if entropyAverage {
						if !math.IsNaN(e) {
							avg += e
							total++
						}
					} else {
						fmt.Println(fmt.Sprintf("%d\t%d\t%.3f", nb, i, e))
					}
				}
			}
			if entropyAverage {
				fmt.Println(fmt.Sprintf("%d\t%.3f", nb, avg/float64(total)))
			}
			nb++
		}
	},
}

func init() {
	computeCmd.AddCommand(entropyCmd)
	entropyCmd.PersistentFlags().BoolVarP(&entropyAverage, "average", "a", false, "Compute only the average entropy of input alignment")
}