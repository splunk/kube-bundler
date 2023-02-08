// Copyright (c) 2021-2021 Splunk, Inc. All rights reserved.
package resources

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// JobResource is a resource for kubernetes Deployments
type JobResource struct {
	clientset   kubernetes.Interface
	category    string
	serviceName string
	name        string
	namespace   string

	// Internal fetch state
	availableReplicas int
	totalReplicas     int
}

func NewJob(clientset kubernetes.Interface, category, serviceName, name, namespace string) DeployableResource {
	return &JobResource{
		clientset:   clientset,
		category:    category,
		serviceName: serviceName,
		name:        name,
		namespace:   namespace,
	}
}

func (j *JobResource) Category() string {
	return j.category
}

func (j *JobResource) ServiceName() string {
	return j.serviceName
}

func (j *JobResource) Name() string {
	return j.name
}

func (j *JobResource) Fetch() error {
	jobInterface := j.clientset.BatchV1().Jobs(j.namespace)
	job, err := jobInterface.Get(context.TODO(), j.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, "Couldn't get job")
	}

	j.availableReplicas = int(*job.Spec.Completions)
	j.totalReplicas = int(*job.Spec.Parallelism)
	return nil
}

func (j *JobResource) Restart(force bool) error {
	return nil
}

func (j *JobResource) Scale(replicas int) error {
	return nil
}

func (j *JobResource) Delete() error {
	return nil
}

func (j *JobResource) Logs(opts LogOpts) (map[string]LogInfo, error) {
	m := make(map[string]LogInfo)

	// Get job
	job, err := j.clientset.BatchV1().Jobs(j.namespace).Get(context.TODO(), j.name, DefaultGetOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch job "+j.name)
	}

	// Get matching pods
	podInterface := j.clientset.CoreV1().Pods(j.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(job.Spec.Selector.MatchLabels).String(),
	}
	podList, err := podInterface.List(context.TODO(), listOptions)
	if err != nil {
		return nil, errors.Wrapf(err, "error initializing pod interface")
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

			prefix := fmt.Sprintf("[%s.%s.%s]", j.serviceName, pod.Name, containerName)
			logInfo := LogInfo{
				Service:   j.serviceName,
				Pod:       pod.Name,
				Container: containerName,
				Logs:      podLogs,
			}
			m[prefix] = logInfo
		}
	}

	return m, nil
}

func (j *JobResource) Status() (map[string]StatusInfo, error) {
	m := make(map[string]StatusInfo)

	// Get job
	job, err := j.clientset.BatchV1().Jobs(j.namespace).Get(context.TODO(), j.name, DefaultGetOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch job "+j.name)
	}

	// Get matching pods
	podInterface := j.clientset.CoreV1().Pods(j.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(job.Spec.Selector.MatchLabels).String(),
	}
	podList, err := podInterface.List(context.TODO(), listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch podlist for job "+j.name)
	}
	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			statusInfo := StatusInfo{
				Service:        j.serviceName,
				Pod:            pod.Name,
				Container:      containerStatus.Name,
				NodeName:       pod.Spec.NodeName,
				ContainerType:  "init",
				ContainerState: getContainerState(containerStatus),
				Restarts:       int(containerStatus.RestartCount),
			}
			// Generate a key that will provide consistent lexigraphical ordering
			key := fmt.Sprintf("[%d.%s.%s.%s]", 0, j.serviceName, pod.Name, containerStatus.Name)
			m[key] = statusInfo
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			statusInfo := StatusInfo{
				Service:        j.serviceName,
				Pod:            pod.Name,
				Container:      containerStatus.Name,
				NodeName:       pod.Spec.NodeName,
				ContainerType:  "app",
				ContainerState: getContainerState(containerStatus),
				Restarts:       int(containerStatus.RestartCount),
			}
			// Generate a key that will provide consistent lexigraphical ordering
			key := fmt.Sprintf("[%d.%s.%s.%s]", 1, j.serviceName, pod.Name, containerStatus.Name)
			m[key] = statusInfo
		}
	}

	return m, nil
}

func (j *JobResource) AvailableReplicas() int {
	return j.availableReplicas
}

func (j *JobResource) TotalReplicas() int {
	return j.totalReplicas
}

func (j *JobResource) NeedsQuorum() bool {
	return false
}

func (j *JobResource) Wait(timeout time.Duration) error {
	jobInterface := j.clientset.BatchV1().Jobs(j.namespace)
	fmt.Print("Waiting for ", timeout, " for ", j.name, " to complete...")

	for start := time.Now(); time.Since(start) < timeout; {
		job, err := jobInterface.Get(context.TODO(), j.name, DefaultGetOptions)
		if err != nil {
			log.WithField("error", err).Debug("Error fetching job state")
			time.Sleep(5 * time.Second)
			continue
		}
		jobConditions := job.Status.Conditions
		if len(jobConditions) > 0 {
			latestStatus := jobConditions[len(jobConditions)-1].Type
			if latestStatus == batchv1.JobComplete {
				fmt.Println()
				log.Debug("Job is complete")
				return nil
			}
		}
		fmt.Print(".")
		time.Sleep(5 * time.Second)
	}
	fmt.Println()
	return errors.New("timeout expired waiting for job")
}
