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
	"fmt"

	"github.com/spf13/cobra"
)

type Info struct {
	FullVersion     string
	SemanticVersion string
	BuildDateTime   string
	PipelineID      string
	GitShortSHA     string
	BuildOS         string
}

func (i Info) String() string {
	return fmt.Sprintf("Full Version: %s\nSemantic Version: %s\nBuild Date: %s\nBuild Pipeline ID: %s\nGit SHA: %s\nBuild OS: %s",
		i.FullVersion, i.SemanticVersion, i.BuildDateTime, i.PipelineID, i.GitShortSHA, i.BuildOS)
}

var (
	VInfo Info
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print tool version information",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(VInfo)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
