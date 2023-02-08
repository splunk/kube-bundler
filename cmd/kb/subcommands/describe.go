package subcommands

import "github.com/spf13/cobra"

func init() {
	rootCmd.AddCommand(describeCmd)
}

var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Describe resources",
	Long:  "Describe resources",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}
