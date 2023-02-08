package main

import (
	sub "github.com/splunk/kube-bundler/cmd/kb/subcommands"
	"github.com/splunk/kube-bundler/cmd/kb/subcommands"
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
