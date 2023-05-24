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
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/splunk/kube-bundler/managers"
)

const (
	timeout                = 30
	healthStatusNS         = "monitoring"
	healthStatusService    = "health-status:8080"
	appStatusEndpoint      = "application-status"
	clusterStatusEndpoint  = "cluster-status"
	resourceStatusEndpoint = "resource-status"
)

var appStatusMap = map[int]string{0: "down", 1: "up"}

func init() {
	statusCmd.AddCommand(healthStatusCmd)
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(statusCmd)
	serverCmd.AddCommand(healthStatusCmd)
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start server to get resource statuses",
	Long:  "Start server to get resource statuses",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var healthStatusCmd = &cobra.Command{
	Use:   "health-status",
	Short: "Start health-status server",
	Long:  "Start health-status server",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := setup()
		sm := managers.NewStatusManager(c)

		return sm.HealthStatus()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get bundle status",
	Long:  "Get bundle status",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := setup()
		return printStatus(c)
	},
}

var applicationStatusCmd = &cobra.Command{
	Use:   "applications",
	Short: "Get application status",
	Long:  "Get application status",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := setup()
		return printApplicationStatus(c)
	},
}

var resourceStatusCmd = &cobra.Command{
	Use:   "resources",
	Short: "Get resource status",
	Long:  "Get resource status",
	RunE: func(cmd *cobra.Command, args []string) error {
		c := setup()

		resources, err := getResourceStatus(c)
		if err != nil {
			return err
		}

		return printResourceStatus(c, resources)
	},
}

func printStatus(kbClient managers.KBClient) error {
	c := setup()

	errList := make([]string, 0)
	w := tabwriter.NewWriter(os.Stdout, 1, 3, 3, ' ', 0)
	fmt.Fprintf(w, "\nSTATUS CHECK\tREADY")

	// Getting application health check summary
	apps, err := getApplicationStatus(c)
	if err != nil {
		fmt.Fprintf(w, "\napplications\terror")
	} else {
		appHealthyCount := 0
		appTotalCount := 0
		for _, appStatus := range apps.AppStatuses {
			appTotalCount += len(appStatus.Endpoints)
			for _, endpoint := range appStatus.Endpoints {
				if appStatusMap[endpoint.Status] == "down" {
					errList = append(errList, fmt.Sprintf("* application \"%s\" is %s", appStatus.App, appStatusMap[endpoint.Status]))
					continue
				}
				appHealthyCount++
			}
		}
		fmt.Fprintf(w, "\napplications\t%d/%d", appHealthyCount, appTotalCount)
	}

	// Getting resource health check summary
	resources, err := getResourceStatus(c)
	if err != nil {
		fmt.Fprintf(w, "\nresources\terror")
	} else {
		resourceHealthyCount := 0
		for _, r := range resources {
			if r.Status != "up" {
				errList = append(errList, fmt.Sprintf("* resource \"%s\" is %s", r.Name, r.Status))
				continue
			}
			resourceHealthyCount++
		}
		fmt.Fprintf(w, "\nresources\t%d/%d", resourceHealthyCount, len(resources))
	}

	fmt.Fprintln(w)
	// Printing all health check errors
	if len(errList) > 0 {
		fmt.Fprintf(w, "\nSTATUS DETAILS\n")
		for _, e := range errList {
			fmt.Fprintf(w, "%s\n", e)
		}
	}
	w.Flush()
	return nil
}

func getHealthStatus(kbClient managers.KBClient, endpoint string) ([]byte, error) {
	resp, err := kbClient.Interface.CoreV1().RESTClient().Get().
		Namespace(healthStatusNS).
		Resource("services").
		Name(healthStatusService).
		Suffix(endpoint).
		SubResource("proxy").
		Timeout(timeout * time.Second).
		DoRaw(context.TODO())
	if err != nil {
		return []byte{}, errors.Wrap(err, "Failed to retrieve health status results")
	}
	return resp, nil
}

func getApplicationStatus(kbClient managers.KBClient) (managers.AppStatusResponse, error) {
	resp, err := getHealthStatus(kbClient, appStatusEndpoint)
	if err != nil {
		return managers.AppStatusResponse{}, errors.Wrap(err, "Failed to retrieve application status results")
	}

	var data managers.AppStatusResponse
	err = json.Unmarshal(resp, &data)
	if err != nil {
		return managers.AppStatusResponse{}, errors.Wrap(err, "Couldn't unmarshal application status results")
	}
	return data, nil
}

func getResourceStatus(kbClient managers.KBClient) ([]managers.ResourceStatus, error) {
	resp, err := getHealthStatus(kbClient, resourceStatusEndpoint)
	if err != nil {
		return []managers.ResourceStatus{}, errors.Wrap(err, "Failed to retrieve resource status results")
	}

	var data []managers.ResourceStatus
	err = json.Unmarshal(resp, &data)
	if err != nil {
		return []managers.ResourceStatus{}, errors.Wrap(err, "Couldn't unmarshal resource status results")
	}
	return data, nil
}

func printApplicationStatus(c managers.KBClient) error {
	data, err := getApplicationStatus(c)
	if err != nil {
		return errors.Wrap(err, "Failed to print application status results")
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 5, 3, ' ', 0)
	fmt.Fprintf(w, "\nAPPLICATION\tENDPOINT\tSTATUS\tAGE\n")

	sort.Slice(data.AppStatuses, func(i, j int) bool {
		return data.AppStatuses[i].App < data.AppStatuses[j].App
	})
	for _, appStatus := range data.AppStatuses {
		for _, endpoint := range appStatus.Endpoints {
			age := time.Now().Sub(appStatus.LastUpdated).Round(time.Second)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", appStatus.App, endpoint.Endpoint, appStatusMap[endpoint.Status], age)
		}
	}
	w.Flush()
	return nil
}

func printResourceStatus(c managers.KBClient, resources []managers.ResourceStatus) error {
	resources = getUniqueResources(resources)
	resources = sortResourceStatus(resources)

	w := tabwriter.NewWriter(os.Stdout, 1, 3, 3, ' ', 0)
	fmt.Fprintf(w, "\nCATEGORY\tSERVICE\tRESOURCE\tTYPE\tSTATUS\tERRORS\tAGE\n")

	for _, r := range resources {
		age := time.Now().Sub(r.LastUpdated).Round(time.Second)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Category, r.Service, r.Name, r.Type, r.Status, r.Error, age)
	}
	w.Flush()
	return nil
}

func sortResourceStatus(resources []managers.ResourceStatus) []managers.ResourceStatus {
	sort.Slice(resources, func(i, j int) bool {
		var c1, c2, c3 int
		c1 = strings.Compare(resources[i].Category, resources[j].Category)
		c2 = strings.Compare(resources[i].Service, resources[j].Service)
		c3 = strings.Compare(resources[i].Name, resources[j].Name)
		if c1 != 0 {
			return c1 < 0
		} else if c2 != 0 {
			return c2 < 0
		} else if c3 != 0 {
			return c3 < 0
		}
		return false
	})
	return resources
}

func getUniqueResources(r []managers.ResourceStatus) []managers.ResourceStatus {
	// remove duplicates
	uniqueResources := make([]managers.ResourceStatus, 0)
	resourceMap := make(map[managers.ResourceStatus]bool)
	for _, r := range r {
		if _, ok := resourceMap[r]; !ok {
			resourceMap[r] = true
			uniqueResources = append(uniqueResources, r)
		}
	}
	return uniqueResources
}
