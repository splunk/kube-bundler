package subcommands

import (
	"context"

	"github.com/splunk/kube-bundler/managers"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(registerCmd)
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a bundle",
	Long:  "Register a bundle",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return register(args)
	},
}

func register(bundles []string) error {
	c := setup()

	ctx := context.Background()
	mgr := managers.NewRegisterManager(c)

	bundleSource := managers.NewMultiFileSource(bundles)
	bundleRefs := make([]managers.BundleRef, 0)
	for _, bundle := range bundles {
		bundleFile, err := managers.NewBundleFromFile(bundle)
		if err != nil {
			return errors.Wrapf(err, "couldn't load bundle '%s'", bundle)
		}
		bundleRefs = append(bundleRefs, managers.BundleRef{Name: bundleFile.Name, Version: bundleFile.Version})
	}

	_, err := mgr.RegisterAll(ctx, bundleRefs, bundleSource, defaultNamespace)
	if err != nil {
		return errors.Wrapf(err, "couldn't register bundles")
	}

	return nil
}
