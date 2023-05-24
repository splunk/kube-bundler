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
	"io"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/kube-bundler/managers"
)

func init() {
	diffManifestCmd.Flags().BoolVarP(&showLogs, "show-logs", "l", false, "show deploy and smoketest logs")
	diffManifestCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 90, "timeout in seconds")

	diffBundleCmd.Flags().IntVarP(&timeoutSeconds, "timeout", "t", 60, "timeout in seconds")

	diffCmd.AddCommand(diffManifestCmd)
	diffCmd.AddCommand(diffBundleCmd)
	rootCmd.AddCommand(diffCmd)
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Diff resources",
	Long:  "Diff resources",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var diffManifestCmd = &cobra.Command{
	Use:   "manifest",
	Short: "Diff manifests",
	Long:  "Diff manifests",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return diffManifest(args)
	},
}

func diffManifest(manifests []string) error {
	c := setup()

	ctx := context.Background()
	manifestMgr := managers.NewManifestManager(c)

	for _, manifest := range manifests {
		manifestRef := managers.ManifestReference{
			Name:      manifest,
			Namespace: defaultNamespace,
		}

		err := manifestMgr.Diff(ctx, manifestRef, time.Duration(timeoutSeconds)*time.Second)
		if err != nil {
			return errors.Wrapf(err, "couldn't diff manifest '%s'", manifest)
		}
	}

	return nil
}

var diffBundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Diff an install to see what changes will be applied during deploy",
	Long:  "Diff an install to see what changes will be applied during deploy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return diffBundle(args)
	},
}

func diffBundle(installs []string) error {
	c := setup()

	ctx := context.Background()
	deployMgr := managers.NewDeployManager(c)

	for _, installName := range installs {
		installRef := managers.InstallReference{
			Name:      installName,
			Namespace: defaultNamespace,
		}

		deployOpts := managers.DeployOpts{
			Action:  managers.ActionDiff,
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		}
		err := deployMgr.Deploy(ctx, installRef, deployOpts, showLogs)
		if err != nil {
			return errors.Wrapf(err, "couldn't execute deploy for %q", installName)
		}

		logs, err := deployMgr.GetLogs(ctx, installRef)
		if err != nil {
			return errors.Wrap(err, "couldn't get deploy logs")
		}

		_, err = io.Copy(os.Stdout, logs)
		if err != nil {
			return errors.Wrap(err, "couldn't print deploy logs")
		}

		logs.Close()
	}

	return nil
}
