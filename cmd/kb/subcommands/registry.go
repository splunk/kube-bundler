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
	deleteCmd.AddCommand(deleteRegistryCmd)
}

var deleteRegistryCmd = &cobra.Command{
	Use:   "registry",
	Short: "Delete registries",
	Long:  "Delete registries",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return deleteRegistry(args)
	},
}

func deleteRegistry(registries []string) error {
	c := setup()

	ctx := context.Background()
	registryMgr := managers.NewRegistryManager(c)

	for _, registryName := range registries {
		err := registryMgr.Delete(ctx, managers.RegistryRef{Name: registryName, Namespace: defaultNamespace})
		if err != nil {
			return errors.Wrapf(err, "couldn't delete registry '%s'", registryName)
		}
	}

	return nil
}
