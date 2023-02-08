package subcommands

import (
	"context"
	"fmt"
	"time"

	"github.com/splunk/kube-bundler/managers"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	smoketestManifestCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 300, "timeout in seconds")
	smoketestManifestCmd.Flags().BoolVarP(&showLogs, "show-logs", "l", false, "show deploy logs")

	smoketestBundleCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 300, "timeout in seconds")
	smoketestBundleCmd.Flags().BoolVarP(&showLogs, "show-logs", "l", false, "show deploy logs")

	smoketestCmd.AddCommand(smoketestManifestCmd)
	smoketestCmd.AddCommand(smoketestBundleCmd)
	rootCmd.AddCommand(smoketestCmd)
}

var smoketestCmd = &cobra.Command{
	Use:   "smoketest",
	Short: "Smoketest resources",
	Long:  "Smoketest resources",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var smoketestManifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Smoketest manifests",
	Long:  "Smoketest manifests",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return smoketestManifest(args)
	},
}

func smoketestManifest(manifests []string) error {
	c := setup()

	ctx := context.Background()
	manifestMgr := managers.NewManifestManager(c)

	for _, manifest := range manifests {
		manifestRef := managers.ManifestReference{
			Name:      manifest,
			Namespace: defaultNamespace,
		}

		err := manifestMgr.Smoketest(ctx, manifestRef, showLogs, time.Duration(timeoutSeconds)*time.Second)
		if err != nil {
			return errors.Wrapf(err, "couldn't deploy manifest '%s'", manifest)
		}
	}

	return nil
}

var smoketestBundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Smoketest Applications",
	Long:  "Smoketest Applications",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return smoketestBundle(args)
	},
}

func smoketestBundle(args []string) error {
	c := setup()

	ctx := context.Background()

	for _, installName := range args {
		installRef := managers.InstallReference{Name: installName, Namespace: defaultNamespace}
		smoketestMgr := managers.NewSmoketestManager(c)

		err := smoketestMgr.Smoketest(ctx, installRef, showLogs, time.Duration(timeoutSeconds)*time.Second)
		if err != nil {
			return errors.Wrapf(err, "couldn't run smoketest for %q", installName)
		}
	}

	fmt.Printf("Smoketests complete\n")
	return nil
}
