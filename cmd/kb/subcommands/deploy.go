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
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/kube-bundler/managers"
)

var (
	controllerOnly bool
)

func init() {
	deployManifestCmd.Flags().BoolVarP(&showLogs, "show-logs", "l", false, "show deploy and smoketest logs")
	deployManifestCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 90, "timeout in seconds")

	deployBundleCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 90, "timeout in seconds")
	deployBundleCmd.Flags().BoolVarP(&showLogs, "show-logs", "l", false, "show deploy logs")

	deployCmd.AddCommand(deployManifestCmd)
	deployCmd.AddCommand(deployBundleCmd)
	deployCmd.AddCommand(deployRegistryCmd)
	rootCmd.AddCommand(deployCmd)
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy resources",
	Long:  "Deploy resources",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var deployManifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Deploy manifests",
	Long:  "Deploy manifests",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return deployManifest(args)
	},
}

func deployManifest(manifests []string) error {
	c := setup()

	ctx := context.Background()
	manifestMgr := managers.NewManifestManager(c)

	for _, manifest := range manifests {
		manifestRef := managers.ManifestReference{
			Name:      manifest,
			Namespace: defaultNamespace,
		}

		err := manifestMgr.DeploySmoketest(ctx, manifestRef, showLogs, time.Duration(timeoutSeconds)*time.Second)
		if err != nil {
			return errors.Wrapf(err, "couldn't deploy manifest '%s'", manifest)
		}
	}

	return nil
}

var deployBundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Deploy Applications",
	Long:  "Deploy Applications",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return deployBundle(args)
	},
}

func deployBundle(installs []string) error {
	c := setup()

	ctx := context.Background()
	deploySmoketestMgr := managers.NewDeploySmoketestManager(c)

	for _, installName := range installs {
		installRef := managers.InstallReference{
			Name:      installName,
			Namespace: defaultNamespace,
		}
		err := deploySmoketestMgr.DeploySmoketest(ctx, installRef, showLogs, time.Duration(timeoutSeconds)*time.Second)
		if err != nil {
			return err
		}
	}

	return nil
}

var deployRegistryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Deploy registries",
	Long:  "Deploy registries",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return deployRegistry(args)
	},
}

func deployRegistry(registries []string) error {
	c := setup()

	ctx := context.Background()
	registryMgr := managers.NewRegistryManager(c)

	for _, registryName := range registries {
		err := registryMgr.Deploy(ctx, managers.RegistryRef{Name: registryName, Namespace: defaultNamespace})
		if err != nil {
			return errors.Wrapf(err, "couldn't deploy registry '%s'", registryName)
		}
	}

	return nil
}
