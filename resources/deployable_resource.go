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
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	DefaultGetOptions    = metav1.GetOptions{}
	defaultPatchOptions  = metav1.PatchOptions{}
	DefaultUpdateOptions = metav1.UpdateOptions{}
	DefaultDeleteOptions = metav1.DeleteOptions{}
	forceDeleteOptions   = metav1.DeleteOptions{
		GracePeriodSeconds: &zero,
	}
	zero int64 = 0
)

const (
	crashLoopBackOff = "CrashLoopBackOff"
	completed        = "Completed"
)

type LogOpts struct {
	Lines    int
	Follow   bool
	InitOnly bool
}

type LogInfo struct {
	Service   string
	Pod       string
	Container string
	Logs      io.ReadCloser
}

type StatusInfo struct {
	Service        string
	Pod            string
	Container      string
	NodeName       string
	ContainerType  string
	ContainerState string
	Restarts       int
}

// DeployableResource abstracts the API differences between Deployments, Statefulsets, Jobs, etc
type DeployableResource interface {
	// Category returns the category name for this resource (auth, storage, ingest, etc)
	Category() string

	// ServiceName returns the installName for this resource
	ServiceName() string

	// Name returns the resource name
	Name() string

	// Fetch the resource from kubernetes
	Fetch() error

	// Restart the resource.
	// When force=false, the restart is graceful.
	// When force=true, all pods are deleted immediately and service is interrupted
	Restart(force bool) error

	// Scale the resource
	Scale(replicas int) error

	// Delete the resource
	Delete() error

	// Logs returns a map of io.ReadClosers that have log streams. Consumers of this
	// method should read the entire stream for each map entry and issue Close() when done
	Logs(opts LogOpts) (map[string]LogInfo, error)

	// Status returns the status of each container such as restarts and exit status
	Status() (map[string]StatusInfo, error)

	// AvailableReplicas returns the number of currently available replicas
	AvailableReplicas() int

	// TotalReplicas returns the desired number of replicas
	TotalReplicas() int

	// NeedsQuorum determines whether (n/2)+1 replicas are necessary for the service to operate
	NeedsQuorum() bool

	// Wait waits for the resource to be fully rolled out
	Wait(timeout time.Duration) error
}

// Utility functions

// Get container names for the most relevant containers:
// * If initOnly=true, return all init containers
// * If init containers haven't completed, return all init containers
// * If init containers have completed, return all regular containers
func getContainerNames(initOnly bool, pod *corev1.Pod) []string {
	podStatus := pod.Status
	var initContainers []string
	failedOrInProgressInitContainers := false
	for _, initContainerStatus := range podStatus.InitContainerStatuses {
		initContainers = append(initContainers, initContainerStatus.Name)

		// "if !initContainerStatus.Ready" doesn't work for jobs, since they're "Completed"
		waiting := initContainerStatus.State.Waiting
		if waiting != nil && waiting.Reason == crashLoopBackOff {
			failedOrInProgressInitContainers = true
		} else {
			terminated := initContainerStatus.State.Terminated
			if terminated != nil && terminated.ExitCode != 0 {
				failedOrInProgressInitContainers = true
			}
		}
	}
	if failedOrInProgressInitContainers {
		return initContainers
	}
	if initOnly {
		return initContainers
	}

	var containers []string
	for _, containerStatus := range podStatus.ContainerStatuses {
		containers = append(containers, containerStatus.Name)
	}

	return containers
}

func getContainerState(containerStatus corev1.ContainerStatus) string {
	if containerStatus.State.Running != nil {
		return "Running"
	}
	if containerStatus.State.Waiting != nil {
		waiting := containerStatus.State.Waiting
		if waiting.Reason == crashLoopBackOff {
			return crashLoopBackOff
		}
		return "Pending"
	}
	if containerStatus.State.Terminated != nil {
		terminated := containerStatus.State.Terminated
		if terminated.Reason == completed {
			return completed
		}
		return fmt.Sprintf("Terminated (exit code %d)", terminated.ExitCode)
	}
	return "Unknown"
}
