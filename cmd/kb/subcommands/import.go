/*
   Copyright 2023 Splunk Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package subcommands

import (
	"context"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/kube-bundler/managers"
)

var (
	hostArg     string
	registryArg string
	sourceArg   string
	destDirArg  string
)

func init() {
	importBundleCmd.Flags().StringVarP(&registryArg, "registry", "r", "", "name of registry to import into")
	importBundleCmd.Flags().StringVarP(&destDirArg, "dest-dir", "d", "", "base directory to import into; use for fast import directly to local host filesystem")
	importBundleCmd.Flags().StringVarP(&hostArg, "host", "h", "", "host IP of the node that is running the registry pod to import into")
	//importBundleCmd.Flags().StringVarP(&sourceArg, "source", "s", "", "name of source to import from")
	importManifestCmd.Flags().StringVarP(&destDirArg, "dest-dir", "d", "", "base registry directory; use for fast import directly to local host filesystem")
	importManifestCmd.Flags().StringVarP(&hostArg, "host", "h", "", "host IP of the node that is running the registry pod to import into")

	importCmd.AddCommand(importBundleCmd)
	importCmd.AddCommand(importManifestCmd)
	rootCmd.AddCommand(importCmd)
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import resources",
	Long:  "Import resources",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var importBundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Import bundle docker images into a registry",
	Long:  "Import bundle docker images into a registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		return importBundle(registryArg, destDirArg, hostArg, args)
	},
}

func importBundle(registryName string, destDir string, hostArg string, bundles []string) error {
	c := setup()

	ctx := context.Background()
	registryMgr := managers.NewRegistryManager(c)

	bundleSource := managers.NewMultiFileSource(bundles)
	bundleRefs := make([]managers.BundleRef, 0)
	for _, bundle := range bundles {
		bundleFile, err := managers.NewBundleFromFile(bundle)
		if err != nil {
			return errors.Wrapf(err, "couldn't load bundle '%s'", bundle)
		}
		bundleRefs = append(bundleRefs, managers.BundleRef{Name: bundleFile.Name, Version: bundleFile.Version})
	}

	err := registryMgr.Import(ctx, managers.RegistryRef{Name: registryName, Namespace: defaultNamespace}, bundleSource, bundleRefs, destDir, hostArg)
	if err != nil {
		return errors.Wrap(err, "couldn't import bundles")
	}

	return nil
}

var importManifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Import manifest docker images into a registry",
	Long:  "Import manifest docker images into a registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		return importManifest(destDirArg, hostArg, args)
	},
}

func importManifest(destDir string, hostArg string, manifestNames []string) error {
	c := setup()

	ctx := context.Background()
	registryMgr := managers.NewRegistryManager(c)

	for _, manifestName := range manifestNames {
		manifestRef := managers.ManifestReference{Name: manifestName, Namespace: defaultNamespace}
		err := registryMgr.ImportManifest(ctx, manifestRef, destDir, hostArg)
		if err != nil {
			return errors.Wrapf(err, "couldn't import manifest '%s'", manifestName)
		}
	}

	return nil
}
