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
