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
	"net/http"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type StatusManager struct {
	kbClient    KBClient
	resourceMgr *ResourceManager
	configMgr   *ConfigManager
}

func NewStatusManager(kbClient KBClient) *StatusManager {
	return &StatusManager{
		kbClient:    kbClient,
		resourceMgr: NewResourceManager(kbClient),
		configMgr:   NewConfigManager(kbClient),
	}
}

func (sm *StatusManager) HealthStatus() error {
	ctx := context.Background()

	go sm.updateAppList(ctx)
	go sm.updateClusterList(ctx)
	go sm.updateResourceList(ctx)

	// prometheus metrics endpoint
	go func() {
		mux1 := http.NewServeMux()
		mux1.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":9090", mux1)
	}()

	// application and cluster status endpoint
	mux2 := http.NewServeMux()
	mux2.HandleFunc("/application-status", sm.retrieveApplicationStatus)
	mux2.HandleFunc("/cluster-status", sm.retrieveClusterStatus)
	mux2.HandleFunc("/cluster-nodes", sm.retrieveClusterNodes)
	mux2.HandleFunc("/resource-status", sm.retrieveResourceStatus)
	err := http.ListenAndServe(":8080", mux2)
	return errors.Wrap(err, "could not listen on port 8080")
}
