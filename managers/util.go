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
	"fmt"

	"github.com/pkg/errors"
	"github.com/splunk/kube-bundler/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Determine if a string is present in a slice of strings
func stringSliceContains(slice []string, item string) bool {
	for _, i := range slice {
		if i == item {
			return true
		}
	}
	return false
}

func getResourceName(name, suffix string) string {
	resourceName := name
	if suffix != "" {
		resourceName = fmt.Sprintf("%s-%s", name, suffix)
	}
	return resourceName
}

func getNameWithAction(name, action string) string {
	nameWithAction := name
	if action == ActionSmoketest {
		nameWithAction += "-smoketest"
	}
	return nameWithAction
}

func verifyResourceRequirements(ctx context.Context, resourceMgr ResourceManager, client KBClient, manifest v1alpha1.Manifest) error {
	clientset := client.Interface
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "couldn't get cluster nodes")
	}

	for _, n := range nodes.Items {
		if manifest.Spec.CPU != "" {
			minCPU, err := resource.ParseQuantity(manifest.Spec.CPU)
			if err != nil {
				return errors.Wrap(err, "Failed to parse field 'CPU' in manifest")
			}
			availableCPU := n.Status.Allocatable[corev1.ResourceCPU]
			if availableCPU.Value() < minCPU.Value() {
				return errors.Errorf("Insufficient CPU in cluster node %s: available CPU %v is less than required minimum %v", n.Name, availableCPU.Value(), minCPU.Value())
			}
		}

		if manifest.Spec.Memory != "" {
			minMemory, err := resource.ParseQuantity(manifest.Spec.Memory)
			if err != nil {
				return errors.Wrap(err, "Failed to parse field 'Memory' in manifest")
			}
			availableMemory := n.Status.Allocatable[corev1.ResourceMemory]
			if availableMemory.Value() < minMemory.Value() {
				minimumMemoryInGb := minMemory.Value() / 1024 / 1024 / 1024
				actualMemoryInGb := availableMemory.Value() / 1024 / 1024 / 1024
				return errors.Errorf("Insufficient Memory in cluster node %s: available Memory %v is less than required minimum %v", n.Name, actualMemoryInGb, minimumMemoryInGb)
			}
		}
	}
	return nil
}

func verifyNodeRequirements(ctx context.Context, resourceMgr ResourceManager, client KBClient, flavorName string, namespace string) error {
	var flavor v1alpha1.Flavor
	err := resourceMgr.Get(ctx, flavorName, namespace, &flavor)
	if err != nil {
		return errors.Wrapf(err, "couldn't get flavor %q", flavorName)
	}
	clientset := client.Interface
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "couldn't get cluster nodes")
	}
	availableNodes := len(nodes.Items)
	if availableNodes < flavor.Spec.MinimumNodes {
		return errors.Errorf("cluster node count %d is lower than minimum %d required by flavor %s", availableNodes, flavor.Spec.MinimumNodes, flavorName)
	}
	return nil
}
