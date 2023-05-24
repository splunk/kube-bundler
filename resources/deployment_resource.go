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
	"fmt"
	"time"

	"github.com/pkg/errors"
	rolloutstatus "github.com/splunk/kube-bundler/helpers/rolloutstatus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// DeploymentResource is a resource for kubernetes Deployments
type DeploymentResource struct {
	clientset   kubernetes.Interface
	category    string
	serviceName string
	name        string
	namespace   string

	// Internal fetch state
	availableReplicas int
	totalReplicas     int
}

func NewDeployment(clientset kubernetes.Interface, category, serviceName, name, namespace string) DeployableResource {
	return &DeploymentResource{
		clientset:   clientset,
		category:    category,
		serviceName: serviceName,
		name:        name,
		namespace:   namespace,
	}
}

func (d *DeploymentResource) Category() string {
	return d.category
}

func (d *DeploymentResource) ServiceName() string {
	return d.serviceName
}

func (d *DeploymentResource) Name() string {
	return d.name
}

func (d *DeploymentResource) Fetch() error {
	deploymentsClient := d.clientset.AppsV1().Deployments(d.namespace)
	deploy, err := deploymentsClient.Get(context.TODO(), d.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, "Couldn't get deployment")
	}

	d.availableReplicas = int(deploy.Status.ReadyReplicas)
	d.totalReplicas = int(*deploy.Spec.Replicas)
	return nil
}

func (d *DeploymentResource) Restart(force bool) error {
	deploymentInterface := d.clientset.AppsV1().Deployments(d.namespace)
	// Get deployment
	deploy, err := d.clientset.AppsV1().Deployments(d.namespace).Get(context.TODO(), d.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, "couldn't fetch deployment "+d.name)
	}
	if force {
		// Get matching pods
		podInterface := d.clientset.CoreV1().Pods(d.namespace)
		listOptions := metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(deploy.Spec.Selector.MatchLabels).String(),
		}
		err := podInterface.DeleteCollection(context.TODO(), forceDeleteOptions, listOptions)
		if err != nil {
			return errors.Wrap(err, "Couldn't delete pods for the deployment: "+d.name)
		}
	} else {
		currTime := time.Now().Format(time.RFC3339)
		payload := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, currTime)

		_, err := deploymentInterface.Patch(
			context.TODO(),
			d.name,
			types.StrategicMergePatchType,
			[]byte(payload),
			defaultPatchOptions,
		)
		return err
	}
	return nil
}

func (d *DeploymentResource) Scale(replicas int) error {
	deploymentsClient := d.clientset.AppsV1().Deployments(d.namespace)
	deployScale, err := deploymentsClient.GetScale(context.TODO(), d.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("Failed to get latest status of Deployment: %q\n", d.name))
	}

	ds := *deployScale
	ds.Spec.Replicas = int32(replicas)

	_, err = deploymentsClient.UpdateScale(context.TODO(), d.name, &ds, DefaultUpdateOptions)

	if err != nil {
		return errors.Wrap(err, "Failed to update replica count of Deployment")
	}
	fmt.Println("deployment.extensions/" + d.name + " scaled")
	return nil
}

func (d *DeploymentResource) Delete() error {
	_, err := d.clientset.AppsV1().Deployments(d.namespace).Get(context.TODO(), d.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, "couldn't fetch deployment "+d.name)
	}

	err = d.clientset.AppsV1().Deployments(d.namespace).Delete(context.TODO(), d.name, DefaultDeleteOptions)
	if err != nil {
		return errors.Wrap(err, "error deleting deployment "+d.name)
	}
	return nil
}

func (d *DeploymentResource) Logs(opts LogOpts) (map[string]LogInfo, error) {
	m := make(map[string]LogInfo)

	// Get deployment
	deploy, err := d.clientset.AppsV1().Deployments(d.namespace).Get(context.TODO(), d.name, DefaultGetOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch deployment "+d.name)
	}

	// Get matching pods
	podInterface := d.clientset.CoreV1().Pods(d.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(deploy.Spec.Selector.MatchLabels).String(),
	}
	podList, err := podInterface.List(context.TODO(), listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing pod interface")
	}
	for _, pod := range podList.Items {
		for _, containerName := range getContainerNames(opts.InitOnly, &pod) {

			podLogOpts := &corev1.PodLogOptions{
				Container: containerName, // empty string will raise error during flag validation
				Follow:    opts.Follow,
			}
			if opts.Lines != -1 {
				numLines := int64(opts.Lines)
				podLogOpts.TailLines = &numLines
			}

			req := podInterface.GetLogs(pod.Name, podLogOpts)
			podLogs, err := req.Stream(context.TODO())
			if err != nil {
				return nil, errors.Wrapf(err, "error opening stream for pod %s container %s", pod.Name, containerName)
			}

			logInfo := LogInfo{
				Service:   d.serviceName,
				Pod:       pod.Name,
				Container: containerName,
				Logs:      podLogs,
			}
			prefix := fmt.Sprintf("[%s.%s.%s]", d.serviceName, pod.Name, containerName)
			m[prefix] = logInfo
		}
	}

	return m, nil
}

func (d *DeploymentResource) Status() (map[string]StatusInfo, error) {
	m := make(map[string]StatusInfo)

	// Get deployment
	deploy, err := d.clientset.AppsV1().Deployments(d.namespace).Get(context.TODO(), d.name, DefaultGetOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch deployment "+d.name)
	}

	// Get matching pods
	podInterface := d.clientset.CoreV1().Pods(d.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(deploy.Spec.Selector.MatchLabels).String(),
	}
	podList, err := podInterface.List(context.TODO(), listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch podlist for deployment "+d.name)
	}
	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			statusInfo := StatusInfo{
				Service:        d.serviceName,
				Pod:            pod.Name,
				Container:      containerStatus.Name,
				NodeName:       pod.Spec.NodeName,
				ContainerType:  "init",
				ContainerState: getContainerState(containerStatus),
				Restarts:       int(containerStatus.RestartCount),
			}
			// Generate a key that will provide consistent lexigraphical ordering
			key := fmt.Sprintf("[%d.%s.%s.%s]", 0, d.serviceName, pod.Name, containerStatus.Name)
			m[key] = statusInfo
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			statusInfo := StatusInfo{
				Service:        d.serviceName,
				Pod:            pod.Name,
				Container:      containerStatus.Name,
				NodeName:       pod.Spec.NodeName,
				ContainerType:  "app",
				ContainerState: getContainerState(containerStatus),
				Restarts:       int(containerStatus.RestartCount),
			}
			// Generate a key that will provide consistent lexigraphical ordering
			key := fmt.Sprintf("[%d.%s.%s.%s]", 1, d.serviceName, pod.Name, containerStatus.Name)
			m[key] = statusInfo
		}
	}

	return m, nil
}

func (d *DeploymentResource) AvailableReplicas() int {
	return d.availableReplicas
}

func (d *DeploymentResource) TotalReplicas() int {
	return d.totalReplicas
}

func (d *DeploymentResource) NeedsQuorum() bool {
	return false
}

func (d *DeploymentResource) Wait(timeout time.Duration) error {
	for start := time.Now(); time.Since(start) < timeout; {
		deploymentsClient := d.clientset.AppsV1().Deployments(d.namespace)
		deploy, err := deploymentsClient.Get(context.TODO(), d.name, DefaultGetOptions)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("Failed to get latest status of Deployment: %q", d.name))
		}

		unstructuredD := &unstructured.Unstructured{}
		unstructuredD.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
		if err != nil {
			return errors.Wrap(err, "Failed to convert unstructued deployment")
		}

		viewer := &rolloutstatus.DeploymentStatusViewer{}

		msg, done, err := viewer.Status(unstructuredD, 0)
		if err != nil {
			return errors.Wrap(err, "Failed to get deployment status")
		}

		fmt.Print(msg)
		if done {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return errors.New("timeout expired waiting for deployment")
}
