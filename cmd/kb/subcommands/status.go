package subcommands

import (
	"github.com/splunk/kube-bundler/managers"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(healthStatusCmd)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start server to get resource statuses",
	Long:  "Start server to get resource statuses",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var healthStatusCmd = &cobra.Command{
	Use:   "health-status",
	Short: "Start health-status server",
	Long:  "Start health-status server",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := setup()
		sm := managers.NewStatusManager(c)

		return sm.HealthStatus()
	},
}
