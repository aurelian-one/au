package todocmd

import "github.com/spf13/cobra"

var Command = &cobra.Command{
	Use: "todo",
}

var getCommand = &cobra.Command{
	Use: "get",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var listCommand = &cobra.Command{
	Use: "list",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var createCommand = &cobra.Command{
	Use: "create",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var editCommand = &cobra.Command{
	Use: "edit",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

var deleteCommand = &cobra.Command{
	Use: "delete",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	Command.AddCommand(
		getCommand,
		listCommand,
		createCommand,
		editCommand,
		deleteCommand,
	)
}
