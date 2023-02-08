package resources

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

// KubegresResource is a resource for kubernetes Kubegres
type KubegresResource struct {
	clientset   kubernetes.Interface
	category    string
	serviceName string
	name        string
	namespace   string

	// Internal fetch state
	availableReplicas int
	totalReplicas     int
}

func NewKubegres(clientset kubernetes.Interface, category, serviceName, name, namespace string) DeployableResource {
	return &KubegresResource{
		clientset:   clientset,
		category:    category,
		serviceName: serviceName,
		name:        name,
		namespace:   namespace,
	}
}

func (k *KubegresResource) getKubegresStatefulSets() ([]DeployableResource, error) {
	statefulSetInterface := k.clientset.AppsV1().StatefulSets(k.namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", k.name),
	}
	statefulSetList, err := statefulSetInterface.List(context.TODO(), listOptions)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't list stateful sets")
	}

	var resourceList []DeployableResource
	for _, sts := range statefulSetList.Items {
		resource := NewStatefulSet(k.clientset, k.category, k.serviceName, sts.Name, sts.Namespace)
		resourceList = append(resourceList, resource)
	}
	return resourceList, nil
}

func (k *KubegresResource) getKubegresClient() (dynamic.ResourceInterface, error) {
	kubeconfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't get kubeconfig")
	}
	client := dynamic.NewForConfigOrDie(kubeconfig)
	resource := schema.GroupVersionResource{
		Group:    "kubegres.reactive-tech.io",
		Version:  "v1",
		Resource: "kubegres",
	}
	return client.Resource(resource).Namespace(k.namespace), nil
}

func (k *KubegresResource) Category() string {
	return k.category
}

func (k *KubegresResource) ServiceName() string {
	return k.serviceName
}

func (k *KubegresResource) Name() string {
	return k.name
}

func (k *KubegresResource) Fetch() error {
	client, err := k.getKubegresClient()
	if err != nil {
		return errors.Wrapf(err, "couldn't get kubegres client")
	}
	kg, err := client.Get(context.TODO(), k.name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "couldn't get kubegres %s", k.name)
	}
	stsList, err := k.getKubegresStatefulSets()
	if err != nil {
		return errors.Wrapf(err, "couldn't get stateful sets in kubegres %s", k.name)
	}
	k.availableReplicas = len(stsList)
	k.totalReplicas = int((kg.Object["spec"].(map[string]interface{})["replicas"]).(int64))
	return nil
}

func (k *KubegresResource) Restart(force bool) error {
	stsList, err := k.getKubegresStatefulSets()
	if err != nil {
		return errors.Wrapf(err, "couldn't get stateful sets in kubegres %s", k.name)
	}
	errOccurred := false
	for _, sts := range stsList {
		if err = sts.Restart(force); err != nil {
			errOccurred = true
			log.WithFields(log.Fields{"err": err, "name": sts.Name(), "namespace": k.namespace}).Warn("failed to restart stateful set")
		}
	}
	if errOccurred {
		return errors.Wrapf(err, "failed to restart every stateful set in kubegres %s", k.name)
	}
	return nil
}

func (k *KubegresResource) Scale(replicas int) error {
	client, err := k.getKubegresClient()
	if err != nil {
		return errors.Wrapf(err, "couldn't get kubegres client")
	}
	kg, err := client.Get(context.TODO(), k.name, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "couldn't get kubegres %s", k.name)
	}

	kg.Object["spec"].(map[string]interface{})["replicas"] = int32(replicas)
	kg, err = client.Update(context.TODO(), kg, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "couldn't scale kubegres %s", k.name)
	}
	return nil
}

func (k *KubegresResource) Delete() error {
	client, err := k.getKubegresClient()
	if err != nil {
		return errors.Wrapf(err, "couldn't get kubegres client")
	}
	err = client.Delete(context.TODO(), k.name, metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "couldn't delete kubegres %s", k.name)
	}
	return nil
}

func (k *KubegresResource) Logs(opts LogOpts) (map[string]LogInfo, error) {
	m := make(map[string]LogInfo)
	errOccurred := false

	stsList, err := k.getKubegresStatefulSets()
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't get stateful sets in kubegres %s", k.name)
	}
	for _, sts := range stsList {
		stsLogs, err := sts.Logs(opts)
		if err != nil {
			errOccurred = true
			log.WithFields(log.Fields{"err": err, "name": sts.Name(), "namespace": k.namespace}).Warn("failed to get logs for stateful set")
			continue
		}
		for k, v := range stsLogs {
			m[k] = v
		}
	}
	if errOccurred {
		return m, errors.Wrapf(err, "failed to get logs for every stateful set in kubegres %s", k.name)
	}
	return m, nil
}

func (k *KubegresResource) Status() (map[string]StatusInfo, error) {
	m := make(map[string]StatusInfo)
	errOccurred := false

	stsList, err := k.getKubegresStatefulSets()
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't get stateful sets in kubegres %s", k.name)
	}
	for _, sts := range stsList {
		stsStatus, err := sts.Status()
		if err != nil {
			errOccurred = true
			log.WithFields(log.Fields{"err": err, "name": sts.Name(), "namespace": k.namespace}).Warn("failed to get status for stateful set")
			continue
		}
		for k, v := range stsStatus {
			m[k] = v
		}
	}
	if errOccurred {
		return m, errors.Wrapf(err, "failed to get status for every stateful set in kubegres %s", k.name)
	}
	return m, nil
}

func (k *KubegresResource) AvailableReplicas() int {
	return k.availableReplicas
}

func (k *KubegresResource) TotalReplicas() int {
	return k.totalReplicas
}

func (k *KubegresResource) NeedsQuorum() bool {
	return false
}

func (k *KubegresResource) Wait(timeout time.Duration) error {
	stsList, err := k.getKubegresStatefulSets()
	if err != nil {
		return errors.Wrapf(err, "couldn't get stateful sets in kubegres %s", k.name)
	}
	for _, sts := range stsList {
		if err := sts.Wait(timeout); err != nil {
			return errors.Wrapf(err, "failed to wait on stateful set %s in kubegres %s", sts.Name(), k.name)
		}
	}
	return nil
}
