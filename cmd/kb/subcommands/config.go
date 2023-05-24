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
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/kube-bundler/managers"
)

func init() {
	configCmd.AddCommand(getConfigCmd)
	configCmd.AddCommand(listConfigCmd)
	configCmd.AddCommand(setConfigCmd)
	configCmd.AddCommand(removeConfigCmd)

	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configure deployments",
	Long:  "Configure deployments",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var getConfigCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a config value",
	Long:  "Get a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return getConfig(args[0], args[1])
	},
}

func getConfig(installName, key string) error {
	c := setup()

	ctx := context.Background()
	configMgr := managers.NewConfigManager(c)
	installRef := managers.InstallReference{
		Name:      installName,
		Namespace: defaultNamespace,
	}

	value, err := configMgr.Get(ctx, installRef, key)
	if err != nil {
		return errors.Wrap(err, "couldn't get config value")
	}

	fmt.Printf("%s\n", value)
	return nil
}

var listConfigCmd = &cobra.Command{
	Use:   "list",
	Short: "List all config values",
	Long:  "List all config values",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return listConfig(args[0])
	},
}

func listConfig(installName string) error {
	c := setup()

	ctx := context.Background()
	configMgr := managers.NewConfigManager(c)
	installRef := managers.InstallReference{
		Name:      installName,
		Namespace: defaultNamespace,
	}

	values, err := configMgr.List(ctx, installRef)
	if err != nil {
		return errors.Wrap(err, "couldn't list config values")
	}

	sortedKeys := make([]string, 0, len(values))
	for key := range values {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		fmt.Printf("%s=%s\n", key, values[key])
	}
	return nil
}

var setConfigCmd = &cobra.Command{
	Use:   "set",
	Short: "Set a config value",
	Long:  "Set a config value",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setConfig(args[0], args[1:])
	},
}

func setConfig(installName string, keyValuePairs []string) error {
	c := setup()

	ctx := context.Background()
	configMgr := managers.NewConfigManager(c)
	installRef := managers.InstallReference{
		Name:      installName,
		Namespace: defaultNamespace,
	}

	for _, keyValue := range keyValuePairs {
		key, value, err := splitKeyValue(keyValue)
		if err != nil {
			return errors.Wrap(err, "couldn't split key/value")
		}

		err = configMgr.Set(ctx, installRef, key, value)
		if err != nil {
			return errors.Wrapf(err, "couldn't set config value '%s'", key)
		}
	}

	return nil
}

var removeConfigCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a config value",
	Long:  "Remove a config value",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return removeConfig(args[0], args[1:])
	},
}

func removeConfig(installName string, keys []string) error {
	c := setup()

	ctx := context.Background()
	configMgr := managers.NewConfigManager(c)
	installRef := managers.InstallReference{
		Name:      installName,
		Namespace: defaultNamespace,
	}

	for _, key := range keys {
		err := configMgr.Remove(ctx, installRef, key)
		if err != nil {
			return errors.Wrapf(err, "couldn't remove key '%s'", key)
		}
	}

	return nil
}

func splitKeyValue(keyValue string) (string, string, error) {
	splits := strings.Split(keyValue, "=")
	if len(splits) == 1 {
		return "", "", fmt.Errorf("expected %s in key=value format", keyValue)
	} else if len(splits) == 2 {
		return splits[0], splits[1], nil
	} else {
		value := strings.Join(splits[1:], "")
		return splits[0], value, nil
	}
}
