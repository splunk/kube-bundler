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
