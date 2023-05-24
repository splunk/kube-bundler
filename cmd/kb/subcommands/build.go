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
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/splunk/kube-bundler/managers"
)

var (
	registryImage string
	anonymousPull bool
	buildArg      []string
)

func init() {
	buildCmd.Flags().StringVarP(&appFile, "file", "f", managers.DefaultAppFile, "application definition file")
	buildCmd.Flags().StringVarP(&registryImage, "registry", "r", managers.DefaultRegistryImage, "registry docker image ref")
	buildCmd.Flags().BoolVarP(&anonymousPull, "allow-anonymous-pull", "", false, "whether to allow anonymous pulling from docker registries")
	buildCmd.Flags().StringArrayVarP(&buildArg, "build-arg", "b", []string{}, "additional build args. Need to be passed as key value pairs separated by = sign. E.g --build-arg key=value")

	rootCmd.AddCommand(buildCmd)
}

func processBuildArgs(argsMap map[string]*string) error {
	log.Info("Processing any build args that are passed in via --build-arg")
	// Extract/parse buildArgs and store them in argsMap
	for _, str := range buildArg {
		s := strings.Split(str, "=")
		key := strings.TrimSpace(s[0])
		value := strings.TrimSpace(s[1])
		argsMap[key] = &value
		log.WithFields(log.Fields{"key": key, "value": value}).Info("Found build-arg")
	}

	return nil
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build a bundle",
	Long:  "Build a bundle",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return build(args[0], appFile)
	},
}

// Create a tarball with the application
func build(dir, filename string) error {
	ctx := context.Background()

	buildMgr, err := managers.NewBuildManager(dir, filename)
	if err != nil {
		return errors.Wrapf(err, "initialization failed for %s", filepath.Join(dir, filename))
	}

	//process build args
	var argsMap = make(map[string]*string)
	err = processBuildArgs(argsMap)
	if err != nil {
		return errors.Wrapf(err, "Failed to process build args")
	}

	err = buildMgr.BuildDeployImage(ctx, dir, argsMap)
	if err != nil {
		return errors.Wrapf(err, "Failed to build/upload deployImage.")
	}

	err = buildMgr.UploadDeployImage(ctx)
	if err != nil {
		return errors.Wrapf(err, "Failed to build/upload deployImage.")
	}

	err = buildMgr.Build(ctx, registryImage, anonymousPull)
	if err != nil {
		return errors.Wrapf(err, "build failed for %s", filepath.Join(dir, filename))
	}

	return nil
}
