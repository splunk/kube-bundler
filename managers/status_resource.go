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

package managers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/splunk/kube-bundler/resources"
)

type ResourceStatus struct {
	Category    string    `json:"category"`
	Service     string    `json:"service"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Error       string    `json:"error"`
	LastUpdated time.Time `json:"last_updated"`
}

var resourceStatusResults []ResourceStatus

var statusMap = map[string]float64{
	"down":     0,
	"stopped":  1,
	"degraded": 2,
	"impacted": 3,
	"up":       4,
}

// The value of the gauge corresponds to the values in statusMap
var resourceStatusCheckGauge = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "resource_status_check",
		Help: "The status of kubernetes resources for every service.",
	}, []string{"name", "type", "service", "category"})

func (sm *StatusManager) updateResourceList(ctx context.Context) {
	for {
		var apps v1alpha1.ApplicationList
		if err := sm.resourceMgr.List(ctx, defaultNamespace, &apps); err != nil {
			log.WithField("err", err).Warn("couldn't get applications")
			time.Sleep(30 * time.Second)
			continue
		}
		var installs v1alpha1.InstallList
		if err := sm.resourceMgr.List(ctx, defaultNamespace, &installs); err != nil {
			log.WithField("err", err).Warn("couldn't get installs")
			time.Sleep(30 * time.Second)
			continue
		}
		applicationMap := make(map[string]v1alpha1.Application)
		for _, i := range apps.Items {
			applicationMap[i.Spec.Name] = i
		}
		resourceStatuses := make([]ResourceStatus, 0)

		for _, i := range installs.Items {
			for _, r := range applicationMap[i.Spec.Application].Spec.Resources {
				// edge case: suffixed installs need suffix added to resource names to avoid ambiguity (e.g. for two postgres installs)
				var name string
				if i.Spec.Suffix != "" {
					name = fmt.Sprintf("%s-%s", r.Name, i.Spec.Suffix)
				} else {
					name = r.Name
				}

				log.WithField("install", i.Name).Debugf("Getting config value 'namespace' from install")
				installRef := InstallReference{
					Name:      i.Name,
					Namespace: defaultNamespace,
				}
				ns, err := sm.configMgr.Get(ctx, installRef, "namespace")
				if err != nil {
					log.WithFields(log.Fields{"resource": name, "err": err.Error()}).Warn("Could not get 'namespace' config option for resource")
					continue
				} else if ns == "" {
					log.WithField("resource", name).Warn("Config option 'namespace' is not defined for resource")
					continue
				}
				log.WithFields(log.Fields{"resource": name, "type": r.Type, "category": r.Category, "namespace": ns}).Debugf("Found resource namespace")

				var deployable resources.DeployableResource
				switch r.Type {
				case "cronjob":
					deployable = resources.NewCronJob(sm.kbClient, r.Category, i.Name, name, ns)
				case "daemonset":
					deployable = resources.NewDaemonSet(sm.kbClient, r.Category, i.Name, name, ns)
				case "deployment":
					deployable = resources.NewDeployment(sm.kbClient, r.Category, i.Name, name, ns)
				case "job":
					deployable = resources.NewJob(sm.kbClient, r.Category, i.Name, name, ns)
				case "kubegres":
					deployable = resources.NewKubegres(sm.kbClient, r.Category, i.Name, name, ns)
				case "statefulset":
					deployable = resources.NewStatefulSet(sm.kbClient, r.Category, i.Name, name, ns)
				default:
					log.WithFields(log.Fields{"resource": name, "type": r.Type, "category": r.Category}).Warn("resource type doesn't match supported types")
					continue
				}
				log.WithField("resource", name).Debugf("Fetching status for resource")
				err = deployable.Fetch()
				resource := ResourceStatus{
					Category:    r.Category,
					Service:     i.Name,
					Name:        name,
					Type:        r.Type,
					Status:      sm.calculateStatus(deployable.AvailableReplicas(), deployable.TotalReplicas(), deployable.NeedsQuorum()),
					Error:       sm.calculateError(err),
					LastUpdated: time.Now(),
				}
				resourceStatusCheckGauge.WithLabelValues(name, r.Type, i.Name, r.Category).Set(statusMap[resource.Status])
				resourceStatuses = append(resourceStatuses, resource)
			}
		}
		resourceStatusResults = resourceStatuses
		time.Sleep(interval * time.Second)
	}
}

// Return a status string based on available and total replica counts
//
// up        - service is running as expected
// down      - service is not functional
// stopped   - service is explicitly turned off
// degraded  - service is operating below replica spec, but may still be functional
// impacted  - service is unable to operate normally due to lost replicas
func (sm *StatusManager) calculateStatus(availableReplicas, totalReplicas int, needsQuorum bool) string {
	if totalReplicas == 0 {
		return "stopped"
	}

	if availableReplicas == 0 {
		return "down"
	}

	if availableReplicas == totalReplicas {
		return "up"
	}

	// Check for a majority of available replicas
	if needsQuorum {
		if availableReplicas >= totalReplicas/2 {
			return "degraded"
		}
		return "impacted"
	}

	// Service is degraded because available and total replicas don't match
	return "degraded"
}

func (sm *StatusManager) calculateError(err error) string {
	if err != nil {
		return err.Error()
	}
	return "-"
}

func (sm *StatusManager) retrieveResourceStatus(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(resourceStatusResults)
}
