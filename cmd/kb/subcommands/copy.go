package subcommands

import (
	"context"
	"os"

	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/splunk/kube-bundler/managers"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var destinationSourceFilename, manifestFilename string

func init() {

	copyManifestCmd.Flags().StringVarP(&manifestFilename, "file", "f", "", "manifest file to be used for copying bundles")
	copyManifestCmd.Flags().StringVarP(&configFilename, "config", "c", "", "configuration source used for bundle copy")
	copyManifestCmd.Flags().StringVarP(&destinationSourceFilename, "destination", "d", "", "source to copy the bundles to")
	copyManifestCmd.Flags().StringVarP(&section, "section", "", "latest", "section prefix used for copy")
	copyManifestCmd.Flags().StringVarP(&release, "release", "", "main", "release prefix used for copy")

	copyBundleCmd.Flags().StringVarP(&configFilename, "config", "c", "", "configuration source used by bundle copy")
	copyBundleCmd.Flags().StringVarP(&destinationSourceFilename, "destination", "d", "", "source to copy the bundles to")
	copyBundleCmd.Flags().StringVarP(&section, "section", "", "latest", "section prefix used for copy")
	copyBundleCmd.Flags().StringVarP(&release, "release", "", "main", "release prefix used for copy")

	copyCmd.AddCommand(copyManifestCmd)
	copyCmd.AddCommand(copyBundleCmd)

	rootCmd.AddCommand(copyCmd)

}

var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "copy bundles or manifests",
	Long:  "copy bundles or manifests",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var copyBundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "copy bundles",
	Long:  "copy bundles",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bundleRefs := make([]managers.BundleRef, 0)
		for _, bundle := range args {
			bundleRefs = append(bundleRefs, managers.BundleRef{Name: bundle, Version: managers.Latest})
		}
		return copyBundles(configFilename, destinationSourceFilename, bundleRefs)
	},
}

var copyManifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "copy bundles in a manifest",
	Long:  "copy bundles in a manifest",
	RunE: func(cmd *cobra.Command, args []string) error {
		return copyManifest(configFilename, destinationSourceFilename, manifestFilename)
	},
}

func copyBundles(configFilename, destinationSourceFileName string, bundleRefs []managers.BundleRef) error {
	ctx := context.Background()
	copyMgr := managers.NewCopyManager()

	f, err := os.Open(configFilename)
	if err != nil {
		return errors.Wrap(err, "couldn't open application definition file")
	}
	defer f.Close()

	df, err := os.Open(destinationSourceFileName)
	if err != nil {
		return errors.Wrap(err, "couldn't open application definition file")
	}
	defer df.Close()

	var fromSourceConfig v1alpha1.Source
	decoder := yaml.NewYAMLOrJSONDecoder(f, 100)
	err = decoder.Decode(&fromSourceConfig)
	if err != nil {
		return errors.Wrap(err, "couldn't decode from source yaml")
	}

	var destinationSourceConfig v1alpha1.Source
	destDecoder := yaml.NewYAMLOrJSONDecoder(df, 100)
	err = destDecoder.Decode(&destinationSourceConfig)
	if err != nil {
		return errors.Wrap(err, "couldn't decode destination source yaml")
	}

	fromSource, err := managers.NewSource(fromSourceConfig.Spec.Type, fromSourceConfig.Spec.Path, fromSourceConfig.Spec.Options, section, release)
	if err != nil {
		return errors.Wrapf(err, "couldn't create source instance '%s'", fromSourceConfig.Name)
	}

	destinationSource, err := managers.NewSource(destinationSourceConfig.Spec.Type, destinationSourceConfig.Spec.Path, destinationSourceConfig.Spec.Options, section, release)
	if err != nil {
		return errors.Wrapf(err, "couldn't create destination instance '%s'", destinationSourceConfig.Name)
	}
	err = copyMgr.Copy(ctx, fromSource, destinationSource, defaultNamespace, bundleRefs)
	if err != nil {
		return errors.Wrapf(err, "couldn't copy bundle")
	}
	return nil
}

func copyManifest(configFilename, destinationSourceFilename, manifestFilename string) error {

	f, err := os.Open(manifestFilename)
	if err != nil {
		return errors.Wrap(err, "couldn't open manifest file")
	}
	defer f.Close()

	var manifest v1alpha1.Manifest
	decoder := yaml.NewYAMLOrJSONDecoder(f, 100)
	err = decoder.Decode(&manifest)
	if err != nil {
		return errors.Wrap(err, "couldn't decode manifest yaml")
	}

	bundleRefs := make([]managers.BundleRef, 0)
	for _, bundle := range manifest.Spec.Bundles {
		bundleRefs = append(bundleRefs, managers.BundleRef{Name: bundle.Name, Version: bundle.Version})
	}

	err = copyBundles(configFilename, destinationSourceFilename, bundleRefs)
	if err != nil {
		return errors.Wrap(err, "couldn't copy bundles in the manifest file")
	}
	return nil
}
