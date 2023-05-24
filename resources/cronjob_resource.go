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
	"sort"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// CronJobResource is a resource for kubernetes CronJobs
type CronJobResource struct {
	clientset   kubernetes.Interface
	category    string
	serviceName string
	name        string
	namespace   string

	// Internal fetch state
	availableReplicas int
	totalReplicas     int
}

func NewCronJob(clientset kubernetes.Interface, category, serviceName, name, namespace string) DeployableResource {
	return &CronJobResource{
		clientset:   clientset,
		category:    category,
		serviceName: serviceName,
		name:        name,
		namespace:   namespace,
	}
}

func (c *CronJobResource) Category() string {
	return c.category
}

func (c *CronJobResource) ServiceName() string {
	return c.serviceName
}

func (c *CronJobResource) Name() string {
	return c.name
}

func (c *CronJobResource) Fetch() error {
	cronJobInterface := c.clientset.BatchV1().CronJobs(c.namespace)
	cronJob, err := cronJobInterface.Get(context.TODO(), c.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, "Couldn't get cronjob")
	}

	if !*cronJob.Spec.Suspend {
		c.availableReplicas = 1
		c.totalReplicas = 1
	} else {
		c.availableReplicas = 0
		c.totalReplicas = 0
	}

	return nil
}

func (c *CronJobResource) Restart(force bool) error {
	return nil
}

func (c *CronJobResource) Scale(replicas int) error {
	cronJobInterface := c.clientset.BatchV1().CronJobs(c.namespace)

	if replicas == 0 {
		payload := `{"spec" : {"suspend" : true }}`

		_, err := cronJobInterface.Patch(
			context.TODO(),
			c.name,
			types.StrategicMergePatchType,
			[]byte(payload),
			defaultPatchOptions,
		)
		if err != nil {
			return errors.Wrap(err, "Failed to suspend cronJob")
		}
		fmt.Println("cronJob " + c.name + " suspended")
	} else if replicas > 0 {
		payload := `{"spec" : {"suspend" : false }}`

		_, err := cronJobInterface.Patch(
			context.TODO(),
			c.name,
			types.StrategicMergePatchType,
			[]byte(payload),
			defaultPatchOptions,
		)
		if err != nil {
			return errors.Wrap(err, "Failed to unsuspend cronJob")
		}
		fmt.Println("cronJob " + c.name + " unsuspended")
	}

	return nil
}

func (c *CronJobResource) Delete() error {
	_, err := c.clientset.BatchV1().CronJobs(c.namespace).Get(context.TODO(), c.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, "couldn't fetch cronJob "+c.name)
	}

	err = c.clientset.BatchV1().CronJobs(c.namespace).Delete(context.TODO(), c.name, DefaultDeleteOptions)
	if err != nil {
		return errors.Wrap(err, "error deleting cronJob "+c.name)
	}
	return nil
}

func (c *CronJobResource) Logs(opts LogOpts) (map[string]LogInfo, error) {
	m := make(map[string]LogInfo)

	// Get cronjob
	cronJob, err := c.clientset.BatchV1().CronJobs(c.namespace).Get(context.TODO(), c.name, DefaultGetOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch cronJob "+c.name)
	}

	// Get list of jobs in cronjob history
	jobInterface := c.clientset.BatchV1().Jobs(c.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(cronJob.Spec.JobTemplate.Labels).String(),
	}
	jobList, err := jobInterface.List(context.TODO(), listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing job interface")
	}

	// Get latest job
	if len(jobList.Items) == 0 {
		return m, nil
	}
	sort.SliceStable(jobList.Items, func(i, j int) bool {
		return jobList.Items[j].CreationTimestamp.Before(&jobList.Items[i].CreationTimestamp)
	})
	latestJob := jobList.Items[0]

	// Get matching pods for latest job
	podInterface := c.clientset.CoreV1().Pods(c.namespace)
	listOptions = metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(latestJob.Spec.Selector.MatchLabels).String(),
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
				Service:   c.serviceName,
				Pod:       pod.Name,
				Container: containerName,
				Logs:      podLogs,
			}
			prefix := fmt.Sprintf("[%s.%s.%s]", c.serviceName, pod.Name, containerName)
			m[prefix] = logInfo
		}
	}

	return m, nil
}

func (c *CronJobResource) Status() (map[string]StatusInfo, error) {
	m := make(map[string]StatusInfo)

	// Get cronjob
	cronJob, err := c.clientset.BatchV1().CronJobs(c.namespace).Get(context.TODO(), c.name, DefaultGetOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch cronJob "+c.name)
	}

	// Get list of jobs in cronjob history
	jobInterface := c.clientset.BatchV1().Jobs(c.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(cronJob.Spec.JobTemplate.Labels).String(),
	}
	jobList, err := jobInterface.List(context.TODO(), listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing job interface")
	}

	// Get latest job
	if len(jobList.Items) == 0 {
		return m, nil
	}
	sort.SliceStable(jobList.Items, func(i, j int) bool {
		return jobList.Items[j].CreationTimestamp.Before(&jobList.Items[i].CreationTimestamp)
	})
	latestJob := jobList.Items[0]

	// Get matching pods for latest job
	podInterface := c.clientset.CoreV1().Pods(c.namespace)
	listOptions = metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(latestJob.Spec.Selector.MatchLabels).String(),
	}
	podList, err := podInterface.List(context.TODO(), listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing pod interface")
	}
	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			statusInfo := StatusInfo{
				Service:        c.serviceName,
				Pod:            pod.Name,
				Container:      containerStatus.Name,
				NodeName:       pod.Spec.NodeName,
				ContainerType:  "init",
				ContainerState: getContainerState(containerStatus),
				Restarts:       int(containerStatus.RestartCount),
			}
			// Generate a key that will provide consistent lexigraphical ordering
			key := fmt.Sprintf("[%d.%s.%s.%s]", 0, c.serviceName, pod.Name, containerStatus.Name)
			m[key] = statusInfo
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			statusInfo := StatusInfo{
				Service:        c.serviceName,
				Pod:            pod.Name,
				Container:      containerStatus.Name,
				NodeName:       pod.Spec.NodeName,
				ContainerType:  "app",
				ContainerState: getContainerState(containerStatus),
				Restarts:       int(containerStatus.RestartCount),
			}
			// Generate a key that will provide consistent lexigraphical ordering
			key := fmt.Sprintf("[%d.%s.%s.%s]", 1, c.serviceName, pod.Name, containerStatus.Name)
			m[key] = statusInfo
		}
	}

	return m, nil
}

func (c *CronJobResource) AvailableReplicas() int {
	return c.availableReplicas
}

func (c *CronJobResource) TotalReplicas() int {
	return c.totalReplicas
}

func (c *CronJobResource) NeedsQuorum() bool {
	return false
}

func (c *CronJobResource) Wait(timeout time.Duration) error {
	cronJobInterface := c.clientset.BatchV1().CronJobs(c.namespace)

	for start := time.Now(); time.Since(start) < timeout; {
		cronJob, err := cronJobInterface.Get(context.TODO(), c.name, DefaultGetOptions)
		if err != nil {
			log.WithField("error", err).Debug("Error fetching cronJob state")
			time.Sleep(5 * time.Second)
			continue
		}
		cronJobSuspended := *cronJob.Spec.Suspend
		if !cronJobSuspended {
			fmt.Println()
			log.Debug("CronJob is ready")
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return errors.New("timeout expired waiting for cronJob")
}
