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
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/splunk/kube-bundler/managers"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var (
	configFilename string
	section        string
	release        string
)

func init() {
	publishCmd.Flags().StringVarP(&configFilename, "config", "c", "", "configuration source used by bundle publish")
	publishCmd.Flags().StringVarP(&section, "section", "", "latest", "section prefix used for publish")
	publishCmd.Flags().StringVarP(&release, "release", "", "main", "release prefix used for publish")

	rootCmd.AddCommand(publishCmd)
}

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Publish one or more bundles to a bundle source",
	Long:  "Publish one or more bundles to a bundle source",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return publish(configFilename, args)
	},
}

func publish(configFilename string, filenames []string) error {
	ctx := context.Background()
	publishMgr := managers.NewPublishManager()

	f, err := os.Open(configFilename)
	if err != nil {
		return errors.Wrap(err, "couldn't open application definition file")
	}
	defer f.Close()

	var sourceConfig v1alpha1.Source
	decoder := yaml.NewYAMLOrJSONDecoder(f, 100)
	err = decoder.Decode(&sourceConfig)
	if err != nil {
		return errors.Wrap(err, "couldn't decode app yaml")
	}

	source, err := managers.NewSource(sourceConfig.Spec.Type, sourceConfig.Spec.Path, sourceConfig.Spec.Options, section, release)
	if err != nil {
		return errors.Wrapf(err, "couldn't create source instance '%s'", sourceConfig.Name)
	}

	err = publishMgr.Publish(ctx, source, defaultNamespace, filenames)
	if err != nil {
		return errors.Wrapf(err, "couldn't publish bundle")
	}

	return nil
}
