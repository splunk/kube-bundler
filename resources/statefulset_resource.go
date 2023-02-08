// Copyright (c) 2021-2021 Splunk, Inc. All rights reserved.
package resources

import (
	"context"
	"fmt"
	"time"

	rolloutstatus "github.com/splunk/kube-bundler/helpers/rolloutstatus"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// StatefulSetResource is a resource for kubernetes StatefulSets
type StatefulSetResource struct {
	clientset   kubernetes.Interface
	category    string
	serviceName string
	name        string
	namespace   string

	// Internal fetch state
	availableReplicas int
	totalReplicas     int
}

func NewStatefulSet(clientset kubernetes.Interface, category, serviceName, name, namespace string) DeployableResource {
	return &StatefulSetResource{
		clientset:   clientset,
		category:    category,
		serviceName: serviceName,
		name:        name,
		namespace:   namespace,
	}
}

func (s *StatefulSetResource) Category() string {
	return s.category
}

func (s *StatefulSetResource) ServiceName() string {
	return s.serviceName
}

func (s *StatefulSetResource) Name() string {
	return s.name
}

func (s *StatefulSetResource) Fetch() error {
	statefulSetClient := s.clientset.AppsV1().StatefulSets(s.namespace)
	statefulSet, err := statefulSetClient.Get(context.TODO(), s.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, "Couldn't get statefulset")
	}

	s.availableReplicas = int(statefulSet.Status.ReadyReplicas)
	s.totalReplicas = int(*statefulSet.Spec.Replicas)
	return nil
}

func (s *StatefulSetResource) Restart(force bool) error {
	statefulSetClient := s.clientset.AppsV1().StatefulSets(s.namespace)
	// Get statefulset
	statefulset, err := s.clientset.AppsV1().StatefulSets(s.namespace).Get(context.TODO(), s.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, "couldn't fetch statefulset "+s.name)
	}
	if force {
		// Get matching pods
		podInterface := s.clientset.CoreV1().Pods(s.namespace)
		listOptions := metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(statefulset.Spec.Selector.MatchLabels).String(),
		}
		err := podInterface.DeleteCollection(context.TODO(), forceDeleteOptions, listOptions)
		if err != nil {
			return errors.Wrap(err, "Couldn't delete pods for the statefulset: "+s.name)
		}
	} else {
		currTime := time.Now().Format(time.RFC3339)
		payload := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, currTime)

		_, err = statefulSetClient.Patch(
			context.TODO(),
			s.name,
			types.StrategicMergePatchType,
			[]byte(payload),
			defaultPatchOptions,
		)
		return err
	}
	return nil
}

func (s *StatefulSetResource) Scale(replicas int) error {
	statefulsetClient := s.clientset.AppsV1().StatefulSets(s.namespace)
	statefulset, err := statefulsetClient.GetScale(context.TODO(), s.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, "couldn't fetch statefulset scale")
	}

	statefulset.Spec.Replicas = int32(replicas)

	_, err = statefulsetClient.UpdateScale(context.TODO(), s.name, statefulset, DefaultUpdateOptions)
	if err != nil {
		return errors.Wrap(err, "couldn't update statefulset scale")
	}

	return nil
}

func (s *StatefulSetResource) Delete() error {
	_, err := s.clientset.AppsV1().StatefulSets(s.namespace).Get(context.TODO(), s.name, DefaultGetOptions)
	if err != nil {
		return errors.Wrap(err, "couldn't fetch statefulset "+s.name)
	}

	err = s.clientset.AppsV1().StatefulSets(s.namespace).Delete(context.TODO(), s.name, DefaultDeleteOptions)
	if err != nil {
		return errors.Wrap(err, "error deleting statefulset "+s.name)

	}
	return nil
}

func (s *StatefulSetResource) Logs(opts LogOpts) (map[string]LogInfo, error) {
	m := make(map[string]LogInfo)

	// Get statefulset
	statefulset, err := s.clientset.AppsV1().StatefulSets(s.namespace).Get(context.TODO(), s.name, DefaultGetOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch statefulset "+s.name)
	}

	// Get matching pods
	podInterface := s.clientset.CoreV1().Pods(s.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(statefulset.Spec.Selector.MatchLabels).String(),
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

			prefix := fmt.Sprintf("[%s.%s.%s]", s.serviceName, pod.Name, containerName)
			logInfo := LogInfo{
				Service:   s.serviceName,
				Pod:       pod.Name,
				Container: containerName,
				Logs:      podLogs,
			}
			m[prefix] = logInfo
		}
	}

	return m, nil
}

func (s *StatefulSetResource) Status() (map[string]StatusInfo, error) {
	m := make(map[string]StatusInfo)

	// Get statefulset
	statefulset, err := s.clientset.AppsV1().StatefulSets(s.namespace).Get(context.TODO(), s.name, DefaultGetOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch statefulset "+s.name)
	}

	// Get matching pods
	podInterface := s.clientset.CoreV1().Pods(s.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(statefulset.Spec.Selector.MatchLabels).String(),
	}
	podList, err := podInterface.List(context.TODO(), listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't fetch podlist for statefulset "+s.name)
	}
	for _, pod := range podList.Items {
		for _, containerStatus := range pod.Status.InitContainerStatuses {
			statusInfo := StatusInfo{
				Service:        s.serviceName,
				Pod:            pod.Name,
				Container:      containerStatus.Name,
				NodeName:       pod.Spec.NodeName,
				ContainerType:  "init",
				ContainerState: getContainerState(containerStatus),
				Restarts:       int(containerStatus.RestartCount),
			}
			// Generate a key that will provide consistent lexigraphical ordering
			key := fmt.Sprintf("[%d.%s.%s.%s]", 0, s.serviceName, pod.Name, containerStatus.Name)
			m[key] = statusInfo
		}

		for _, containerStatus := range pod.Status.ContainerStatuses {
			statusInfo := StatusInfo{
				Service:        s.serviceName,
				Pod:            pod.Name,
				Container:      containerStatus.Name,
				NodeName:       pod.Spec.NodeName,
				ContainerType:  "app",
				ContainerState: getContainerState(containerStatus),
				Restarts:       int(containerStatus.RestartCount),
			}
			// Generate a key that will provide consistent lexigraphical ordering
			key := fmt.Sprintf("[%d.%s.%s.%s]", 1, s.serviceName, pod.Name, containerStatus.Name)
			m[key] = statusInfo
		}
	}

	return m, nil
}

func (s *StatefulSetResource) AvailableReplicas() int {
	return s.availableReplicas
}

func (s *StatefulSetResource) TotalReplicas() int {
	return s.totalReplicas
}

func (s *StatefulSetResource) NeedsQuorum() bool {
	return true
}

func (s *StatefulSetResource) Wait(timeout time.Duration) error {
	for start := time.Now(); time.Since(start) < timeout; {
		statefulSetClient := s.clientset.AppsV1().StatefulSets(s.namespace)
		statefulSet, err := statefulSetClient.Get(context.TODO(), s.name, DefaultGetOptions)
		if err != nil {
			return errors.Wrap(err, "Failed to get statefulset")
		}

		unstructuredS := &unstructured.Unstructured{}
		unstructuredS.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(statefulSet)
		if err != nil {
			return errors.Wrap(err, "Failed to convert unstructured statefulset")
		}

		viewer := &rolloutstatus.StatefulSetStatusViewer{}

		msg, done, err := viewer.Status(unstructuredS, 0)
		if err != nil {
			return errors.Wrap(err, "Failed to get statefulset status")
		}

		fmt.Print(msg)
		if done {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return errors.New("timeout expired waiting for statefulset")
}
