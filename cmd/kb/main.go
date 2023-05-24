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

package main

import (
	"github.com/splunk/kube-bundler/cmd/kb/subcommands"
	sub "github.com/splunk/kube-bundler/cmd/kb/subcommands"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

// These variables are populated by LDFLAGS at build time.
// Their names must match the flags verbatim.
var (
	FullVersion     string
	SemanticVersion string
	BuildDateTime   string
	PipelineID      string
	GitShortSHA     string
	BuildOS         string
)

func main() {

	sub.VInfo = sub.Info{
		FullVersion:     FullVersion,
		SemanticVersion: SemanticVersion,
		BuildDateTime:   BuildDateTime,
		PipelineID:      PipelineID,
		GitShortSHA:     GitShortSHA,
		BuildOS:         BuildOS,
	}
	subcommands.Execute()
}
