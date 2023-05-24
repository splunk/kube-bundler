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
	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/splunk/kube-bundler/managers"
)

func init() {
	installBundleCmd.Flags().BoolVarP(&showLogs, "show-logs", "l", false, "show deploy and smoketest logs")
	installBundleCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 90, "timeout in seconds")
	installBundleCmd.Flags().StringVarP(&registryArg, "registry", "r", "", "name of registry to import into")
	installBundleCmd.Flags().BoolP("force", "f", false, "Force installation even if node count does not meet flavor requirement")
	installManifestCmd.Flags().BoolVarP(&showLogs, "show-logs", "l", false, "show deploy and smoketest logs")
	installManifestCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 90, "timeout in seconds")
	installManifestCmd.Flags().BoolVarP(&skipSmoketests, "skip-smoketests", "", false, "skip smoketests")
	installManifestCmd.Flags().BoolP("force", "f", false, "Force installation even if node count does not meet flavor requirement")

	installCmd.AddCommand(installBundleCmd)
	installCmd.AddCommand(installManifestCmd)

	rootCmd.AddCommand(installCmd)
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install bundles or manifests",
	Long:  "Install bundles or manifests",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var installBundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Install bundle files directly",
	Long:  "Install bundle files directly",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return errors.Wrap(err, "Failed to get force parameter")
		}
		return installBundle(args, force)
	},
}

func installBundle(bundles []string, force bool) error {
	c := setup()

	resourceMgr := managers.NewResourceManager(c)
	ctx := context.Background()
	dockerUrl := ""
	if registryArg != "" {
		var registry v1alpha1.Registry
		err := resourceMgr.Get(ctx, registryArg, defaultNamespace, &registry)
		if err != nil {
			return errors.Wrapf(err, "couldn't get registry %q", registryArg)
		}
		dockerUrl = registry.ClusterUrl()
	}
	registerMgr := managers.NewRegisterManager(c)
	installMgr := managers.NewInstallManager(c)

	bundleSource := managers.NewMultiFileSource(bundles)
	bundleRefs := make([]managers.BundleRef, 0)
	for _, bundle := range bundles {
		bundleFile, err := managers.NewBundleFromFile(bundle)
		if err != nil {
			return errors.Wrapf(err, "couldn't load bundle '%s'", bundle)
		}
		bundleRefs = append(bundleRefs, managers.BundleRef{Name: bundleFile.Name, Version: bundleFile.Version})
	}

	apps, err := registerMgr.RegisterAll(ctx, bundleRefs, bundleSource, defaultNamespace)
	if err != nil {
		return errors.Wrap(err, "couldn't register bundles")
	}

	deploySmoketestMgr := managers.NewDeploySmoketestManager(c)

	for _, app := range apps {
		// TODO: provide ability to set parameters, suffix, and registry
		suffix := ""
		parameters := []v1alpha1.ParameterSpec{}
		install, err := installMgr.Install(ctx, app.Spec.Name, app.Spec.Name, defaultNamespace, app.Spec.Version, suffix, defaultFlavor, dockerUrl, force, parameters)
		if err != nil {
			return errors.Wrapf(err, "could install application %s", app.Spec.Name)
		}

		installRef := managers.InstallReference{
			Name:      install.Name,
			Namespace: defaultNamespace,
		}
		err = deploySmoketestMgr.DeploySmoketest(ctx, installRef, showLogs, time.Duration(timeoutSeconds)*time.Second)
		if err != nil {
			return errors.Wrapf(err, "couldn't deploy install %s", install.Name)
		}
	}

	return nil
}

var installManifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Install manifests",
	Long:  "Install manifests",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, err := cmd.Flags().GetBool("force")
		if err != nil {
			return errors.Wrap(err, "Failed to get force parameter")
		}
		return manifestInstall(args, force)
	},
}

func manifestInstall(manifests []string, force bool) error {
	c := setup()

	ctx := context.Background()
	manifestMgr := managers.NewManifestManager(c)

	for _, manifest := range manifests {
		manifestRef := managers.ManifestReference{
			Name:      manifest,
			Namespace: defaultNamespace,
		}

		err := manifestMgr.Install(ctx, manifestRef, force)
		if err != nil {
			return errors.Wrapf(err, "couldn't install manifest '%s'", manifest)
		}

		if !skipSmoketests {
			err = manifestMgr.DeploySmoketest(ctx, manifestRef, showLogs, time.Duration(timeoutSeconds)*time.Second)
			if err != nil {
				return errors.Wrapf(err, "couldn't deploy manifest '%s'", manifest)
			}
		} else {
			err = manifestMgr.Deploy(ctx, manifestRef, showLogs, time.Duration(timeoutSeconds)*time.Second)
			if err != nil {
				return errors.Wrapf(err, "couldn't deploy manifest '%s'", manifest)
			}
		}
	}

	return nil
}
