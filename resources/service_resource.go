// Copyright (c) 2021-2021 Splunk, Inc. All rights reserved.
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
