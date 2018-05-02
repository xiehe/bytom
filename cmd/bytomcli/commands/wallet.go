package commands

import (
	"os"

	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"

	"github.com/bytom/util"
)

var rescanWalletBlocksCmd = &cobra.Command{
	Use:   "rescan-wallet",
	Short: "rescan wallet blocks",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if _, exitCode := util.ClientCall("/rescan-wallet"); exitCode != util.Success {
			os.Exit(exitCode)
		}
		jww.FEEDBACK.Println("Successfully trigger rescan wallet blocks")
	},
}
