package managers

import (
	"context"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/splunk/kube-bundler/api/v1alpha1"
)

type FlavorManager struct {
	kbClient    KBClient
	resourceMgr *ResourceManager
}

func NewFlavorManager(kbClient KBClient) *FlavorManager {
	return &FlavorManager{
		kbClient:    kbClient,
		resourceMgr: NewResourceManager(kbClient),
	}
}

// Create creates a flavor resource if it does not already exist.
func (fm *FlavorManager) Create(ctx context.Context, flavorName, namespace, antiAffinity string, quorumReplicas, replicationReplicas, statelessReplicas, minNodes int) (*v1alpha1.Flavor, error) {
	var flavor v1alpha1.Flavor

	err := fm.resourceMgr.Get(ctx, flavorName, namespace, &flavor)
	if err == nil {
		log.WithFields(log.Fields{"name": flavorName, "namespace": namespace}).Info("Found existing flavor, skipping creation")
		return &flavor, nil
	}

	flavor.Name = flavorName
	flavor.Namespace = namespace
	flavor.Spec.AntiAffinity = antiAffinity
	flavor.Spec.MinimumNodes = minNodes
	flavor.Spec.StatefulQuorumReplicas = quorumReplicas
	flavor.Spec.StatefulReplicationReplicas = replicationReplicas
	flavor.Spec.StatelessReplicas = statelessReplicas

	err = fm.resourceMgr.Create(ctx, &flavor)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't create flavor %s", flavorName)
	}
	return &flavor, nil
}

// Get returns a flavor resource by name if it exists.
func (fm *FlavorManager) Get(ctx context.Context, flavorName, namespace string) (*v1alpha1.Flavor, error) {
	var flavor v1alpha1.Flavor

	err := fm.resourceMgr.Get(ctx, flavorName, namespace, &flavor)
	if err != nil {
		return nil, errors.Wrapf(err, "Could not find flavor %s", flavorName)
	}
	return &flavor, nil
}
