package devcmd

import "github.com/spf13/cobra"

var Command = &cobra.Command{
	Use: "dev",
}

var dumpCommand = &cobra.Command{
	Use: "dump",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	Command.AddCommand(
		dumpCommand,
	)
}
