package managers

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KBClient provides both a controller-runtime client and a client-go client.
// Some operations like getting pod logs can't be done with controller-runtime.
type KBClient struct {
	client.Client
	kubernetes.Interface
	RestConfig *rest.Config
}
