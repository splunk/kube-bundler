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
	"os"
	"text/tabwriter"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/splunk/kube-bundler/managers"
)

const (
	useApplications = "applications"
)

var (
	// useApplicationAliases is a constant value, but go doesn't yet have constant string slices
	useApplicationAliases = []string{"apps", "app", "application"}
)

func init() {
	getCmd.AddCommand(getApplicationsCmd)
	describeCmd.AddCommand(descApplicationsCmd)
	createCmd.AddCommand(createApplicationCmd)
	deleteCmd.AddCommand(deleteApplicationCmd)
}

var getApplicationsCmd = &cobra.Command{
	Use:     useApplications,
	Aliases: useApplicationAliases,
	Short:   "List Applications",
	Long:    "List Applications",
	RunE: func(cmd *cobra.Command, args []string) error {
		return listApplications(args)
	},
}

func listApplications(args []string) error {
	c := setup()

	ctx := context.Background()
	mgr := managers.NewResourceManager(c)
	var list v1alpha1.ApplicationList
	err := mgr.List(ctx, defaultNamespace, &list)
	if err != nil {
		return errors.Wrap(err, "couldn't list applications")
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 3, 3, ' ', 0)
	fmt.Fprintf(w, "APPLICATION\tNAME\tVERSION\t\n")

	for _, app := range list.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\n", app.Name, app.Spec.Name, app.Spec.Version)
	}

	return w.Flush()
}

var descApplicationsCmd = &cobra.Command{
	Use:     useApplications,
	Aliases: useApplicationAliases,
	Short:   "Describe Applications",
	Long:    "Describe Applications",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return descApplications(args)
	},
}

func descApplications(applications []string) error {
	c := setup()

	ctx := context.Background()
	mgr := managers.NewResourceManager(c)
	var list v1alpha1.ApplicationList
	if len(applications) == 0 {
		err := mgr.List(ctx, defaultNamespace, &list)
		if err != nil {
			return errors.Wrap(err, "couldn't list applications")
		}
	} else {
		var app v1alpha1.Application
		err := mgr.Get(ctx, applications[0], defaultNamespace, &app)
		if err != nil {
			return errors.Wrap(err, "couldn't get applications")
		}
		list.Items = append(list.Items, app)
	}

	for _, app := range list.Items {
		fmt.Printf("Application Name: %s\n", app.Name)
		fmt.Printf("Version: %s\n", app.Spec.Version)
		fmt.Printf("\nParameter Definitions\n==========\n")

		w := tabwriter.NewWriter(os.Stdout, 1, 3, 3, ' ', 0)
		fmt.Fprintf(w, "NAME\tDEFAULT\tDESCRIPTION\n")

		for _, parameterDesc := range app.Spec.ParameterDefinitions {
			fmt.Fprintf(w, "%s\t%s\t%s\n", parameterDesc.Name, parameterDesc.Default, parameterDesc.Description)
		}

		w.Flush()
		fmt.Println()
	}

	return nil
}

var createApplicationCmd = &cobra.Command{
	Use:     useApplications,
	Short:   "Create an application",
	Long:    "Create an application",
	Aliases: useApplicationAliases,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Use 'kb register'\n")
		return nil
	},
}

var deleteApplicationCmd = &cobra.Command{
	Use:     useApplications,
	Aliases: useApplicationAliases,
	Short:   "Delete applications",
	Long:    "Delete applications",
	RunE: func(cmd *cobra.Command, args []string) error {
		return deleteApplications(args)
	},
}

func deleteApplications(applications []string) error {
	c := setup()

	ctx := context.Background()
	mgr := managers.NewResourceManager(c)

	for _, appName := range applications {
		var app v1alpha1.Application
		err := mgr.Delete(ctx, appName, defaultNamespace, &app)
		if err != nil {
			return errors.Wrapf(err, "couldn't delete application %q", appName)
		}
	}
	return nil
}
