package subcommands

import (
	"context"
	"time"

	"github.com/splunk/kube-bundler/managers"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove an install",
	Long:  "Remove an install",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return uninstall(args, force)
	},
}

func init() {
	uninstallCmd.Flags().BoolVarP(&force, "force", "f", false, "Whether to force uninstall")
	uninstallCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 90, "timeout in seconds")
	rootCmd.AddCommand(uninstallCmd)
}

func uninstall(installs []string, forceUninstall bool) error {
	c := setup()

	ctx := context.Background()
	deployMgr := managers.NewDeployManager(c)

	for _, installName := range installs {
		installRef := managers.InstallReference{
			Name:      installName,
			Namespace: defaultNamespace,
		}

		// Run the deploy container with action=delete
		deployOpts := managers.DeployOpts{
			Action:  managers.ActionDelete,
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		}
		err := deployMgr.Deploy(ctx, installRef, deployOpts, showLogs)
		if err != nil && !force {
			return errors.Wrapf(err, "couldn't execute deploy for %q", installName)
		} else if err != nil && force {
			log.WithFields(log.Fields{"err": err, "install": installName}).Error("couldn't execute deploy, continuing anyway")
		}

		// Delete the install
		err = deployMgr.Delete(ctx, installRef)
		if err != nil {
			return errors.Wrapf(err, "couldn't delete install for %q", installName)
		}
	}

	return nil
}
