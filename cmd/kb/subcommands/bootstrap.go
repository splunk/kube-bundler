package subcommands

import (
	"context"

	"github.com/splunk/kube-bundler/managers"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	skipCRDs     bool
	skipBindings bool
	skipFlavors  bool
	skipAirgap   bool
	provider     string
	registryURL  string
	projectID    string
)

func init() {
	bootstrapCmd.Flags().BoolVarP(&skipCRDs, "skip-crds", "", false, "skip deploying CRDs")
	bootstrapCmd.Flags().BoolVarP(&skipBindings, "skip-bindings", "", false, "skip deploying bindings")
	bootstrapCmd.Flags().BoolVarP(&skipFlavors, "skip-flavors", "", false, "skip deploying default flavors")
	bootstrapCmd.Flags().BoolVarP(&skipAirgap, "skip-airgap", "", false, "skip deploying airgap support")

	bootstrapCmd.Flags().StringVarP(&provider, "cluster-provider", "", "K0S", "Specify the cluster provider type for the DSP install either K0S or GKE. Default provider is K0s")
	bootstrapCmd.Flags().StringVarP(&registryURL, "registryURL", "", "gcr.io", "Specify the gcr registry URL")
	bootstrapCmd.Flags().StringVarP(&projectID, "projectID", "", "dsp-streamlio", "Specify project Id for the gcr registry")

	rootCmd.AddCommand(bootstrapCmd)
}

var bootstrapCmd = &cobra.Command{
	Use:     "bootstrap",
	Aliases: []string{"setup"},
	Short:   "Bootstrap kb on this cluster",
	Long:    "Bootstrap kb on this cluster",
	Args:    cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runBootstrap()
	},
}

func runBootstrap() error {
	c := setup()

	ctx := context.Background()
	mgr := managers.NewBootstrapManager(c)

	info := managers.GCRInfo{
		RegistryURL: registryURL,
		ProjectID:   projectID,
	}
	err := mgr.DeployAll(ctx, skipCRDs, skipBindings, skipFlavors, skipAirgap, provider, info)
	if err != nil {
		return errors.Wrap(err, "couldn't run bootstrap")
	}

	return nil
}
