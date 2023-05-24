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

package resources

import (
	"context"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// ServiceResource is a resource for kubernetes Deployments
type ServiceResource struct {
	clientset kubernetes.Interface
	category  string
	name      string
	namespace string

	// Internal fetch state
	service *v1.Service
}

func NewService(clientset kubernetes.Interface, category, name, namespace string) ServiceResource {
	return ServiceResource{
		clientset: clientset,
		category:  category,
		name:      name,
		namespace: namespace,
	}
}

func (d *ServiceResource) NodePorts() []int32 {
	ports := d.service.Spec.Ports
	nodePorts := make([]int32, len(ports))
	for i, port := range ports {
		nodePorts[i] = port.NodePort
	}
	return nodePorts
}

func (d *ServiceResource) Fetch() error {
	servicesClient := d.clientset.CoreV1().Services(d.namespace)
	var err error
	d.service, err = servicesClient.Get(context.TODO(), d.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrapf(err, "Couldn't get service %s", d.name)
	}
	return nil
}

func (d *ServiceResource) Category() string {
	return d.category
}

func (d *ServiceResource) Name() string {
	return d.name
}
