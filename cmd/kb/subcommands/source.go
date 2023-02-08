package subcommands

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/splunk/kube-bundler/managers"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	useSources      = "sources"
	useSourcesAlias = []string{"source"}
	sourceOpts      SourceOpts
)

type SourceOpts struct {
	Name string
	Type string
	Path string
}

func init() {
	createSourceCmd.Flags().StringVarP(&sourceOpts.Name, "name", "n", "", "name of source")
	createSourceCmd.Flags().StringVarP(&sourceOpts.Type, "type", "t", "", "type of source")
	createSourceCmd.Flags().StringVarP(&sourceOpts.Path, "path", "p", "", "path of source")

	getCmd.AddCommand(getSourcesCmd)
	createCmd.AddCommand(createSourceCmd)
	deleteCmd.AddCommand(deleteSourceCmd)
}

var getSourcesCmd = &cobra.Command{
	Use:     useSources,
	Aliases: useSourcesAlias,
	Short:   "Get sources",
	Long:    "Get sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listSources(args)
	},
}

func listSources(args []string) error {
	c := setup()

	ctx := context.Background()
	mgr := managers.NewResourceManager(c)
	var list v1alpha1.SourceList
	err := mgr.List(ctx, defaultNamespace, &list)
	if err != nil {
		return errors.Wrap(err, "couldn't list sources")
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 3, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tTYPE\tPATH\tOPTIONS\n")

	for _, source := range list.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", source.Name, source.Spec.Type, source.Spec.Path, source.Spec.Options)
	}

	return w.Flush()
}

var createSourceCmd = &cobra.Command{
	Use:     useSources,
	Aliases: useSourcesAlias,
	Short:   "Create a source",
	Long:    "Create a source",
	Args:    cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		if sourceOpts.Name == "" {
			return errors.New("empty source name")
		}
		if sourceOpts.Type == "" {
			return errors.New("empty source type")
		}
		if sourceOpts.Path == "" {
			return errors.New("empty source path")
		}

		return createSource(sourceOpts)
	},
}

func createSource(opts SourceOpts) error {
	c := setup()

	ctx := context.Background()
	resourceMgr := managers.NewResourceManager(c)

	var source v1alpha1.Source
	source.Name = opts.Name
	source.Namespace = defaultNamespace
	source.Spec.Type = opts.Type
	source.Spec.Path = opts.Path

	err := resourceMgr.Create(ctx, &source)
	if err != nil {
		return errors.Wrap(err, "couldn't create source")
	}

	return nil
}

var deleteSourceCmd = &cobra.Command{
	Use:     useSources,
	Aliases: useSourcesAlias,
	Short:   "Delete sources",
	Long:    "Delete sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		return deleteSources(args)
	},
}

func deleteSources(installs []string) error {
	c := setup()

	ctx := context.Background()
	mgr := managers.NewResourceManager(c)

	for _, sourceName := range installs {
		var source v1alpha1.Source
		err := mgr.Delete(ctx, sourceName, defaultNamespace, &source)
		if err != nil {
			return errors.Wrapf(err, "couldn't delete source %q", sourceName)
		}
	}
	return nil
}
