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

const (
	useInstalls = "installs"
)

var (
	// useInstallsAlias is a constant value, but go doesn't yet have constant string slices
	useInstallsAlias = []string{"install"}
)

type InstallOpts struct {
	Name    string
	Version string
}

var (
	installOpts InstallOpts
)

func init() {
	createInstallCmd.Flags().StringVarP(&installOpts.Name, "name", "n", "", "application to install")
	createInstallCmd.Flags().StringVarP(&installOpts.Version, "version", "v", "", "version to install")

	getCmd.AddCommand(getInstalls)
	describeCmd.AddCommand(descInstallCmd)
	createCmd.AddCommand(createInstallCmd)
	deleteCmd.AddCommand(deleteInstallCmd)
}

var getInstalls = &cobra.Command{
	Use:     useInstalls,
	Aliases: useInstallsAlias,
	Short:   "List installs",
	Long:    "List installs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listInstalls(args)
	},
}

func listInstalls(args []string) error {
	c := setup()

	ctx := context.Background()
	mgr := managers.NewResourceManager(c)
	var list v1alpha1.InstallList
	err := mgr.List(ctx, defaultNamespace, &list)
	if err != nil {
		return errors.Wrap(err, "couldn't list installs")
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 3, 3, ' ', 0)
	fmt.Fprintf(w, "NAME\tAPPLICATION\tVERSION\t\n")

	for _, install := range list.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\n", install.Name, install.Spec.Application, install.Spec.Version)
	}

	return w.Flush()
}

var descInstallCmd = &cobra.Command{
	Use:     useInstalls,
	Aliases: useInstallsAlias,
	Short:   "Describe installs",
	Long:    "Describe installs",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return describeInstalls(args)
	},
}

func describeInstalls(installs []string) error {
	c := setup()

	ctx := context.Background()
	im := managers.NewInstallManager(c)

	installDescriptions, err := im.Describe(ctx, installs)
	if err != nil {
		return errors.Wrap(err, "failed to describe installs")
	}

	for _, desc := range installDescriptions {
		fmt.Printf("Install Name: %s\n", desc.Name)
		fmt.Printf("Application: %s\n", desc.Application)
		fmt.Printf("Version: %s\n", desc.Version)
		fmt.Printf("\nParameters\n==========\n")

		w := tabwriter.NewWriter(os.Stdout, 1, 3, 3, ' ', 0)
		fmt.Fprintf(w, "NAME\tCURRENT VALUE\tDEFAULT\tDESCRIPTION\n")

		for name, params := range desc.Parameters {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, params.Value, params.Default, params.Description)
		}

		w.Flush()
		fmt.Println()
	}

	return nil
}

var createInstallCmd = &cobra.Command{
	Use:     useInstalls,
	Aliases: useInstallsAlias,
	Short:   "Create an install",
	Long:    "Create an install",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return createInstall(args[0], installOpts)
	},
}

func createInstall(appName string, opts InstallOpts) error {
	c := setup()

	ctx := context.Background()
	installMgr := managers.NewInstallManager(c)

	// TODO: provide ability to set parameters, suffix, and registry
	suffix := ""
	parameters := []v1alpha1.ParameterSpec{}
	_, err := installMgr.Install(ctx, appName, opts.Name, defaultNamespace, opts.Version, suffix, defaultFlavor, "", false, parameters)
	if err != nil {
		return errors.Wrap(err, "couldn't create install")
	}

	return nil
}

var deleteInstallCmd = &cobra.Command{
	Use:     useInstalls,
	Aliases: useInstallsAlias,
	Short:   "Delete installs",
	Long:    "Delete installs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return deleteInstalls(args)
	},
}

// deleteInstalls removes an install without removing its resources
func deleteInstalls(installs []string) error {
	c := setup()

	ctx := context.Background()
	mgr := managers.NewResourceManager(c)

	for _, installName := range installs {
		var install v1alpha1.Install
		err := mgr.Delete(ctx, installName, defaultNamespace, &install)
		if err != nil {
			return errors.Wrapf(err, "couldn't delete install %q", installName)
		}
	}
	return nil
}
