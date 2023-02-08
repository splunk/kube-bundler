package managers

import (
	"context"
	"fmt"
	"time"

	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/splunk/kube-bundler/resources"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type RolloutType string

const (
	Deployment  RolloutType = "deployment"
	Statefulset RolloutType = "statefulset"
	Job         RolloutType = "job"
	Daemonset   RolloutType = "daemonset"
	Kubegres    RolloutType = "kubegres"
	CronJob     RolloutType = "cronjob"
	Service     RolloutType = "service"
)

type RolloutStatusManager struct {
	kbClient    KBClient
	resourceMgr *ResourceManager
}

func NewRolloutStatusManager(c KBClient) *RolloutStatusManager {
	return &RolloutStatusManager{
		kbClient:    c,
		resourceMgr: NewResourceManager(c),
	}
}

func (rsm *RolloutStatusManager) Wait(ctx context.Context, installRef InstallReference, timeout time.Duration) error {
	var install v1alpha1.Install
	err := rsm.resourceMgr.Get(ctx, installRef.Name, installRef.Namespace, &install)
	if err != nil {
		return errors.Wrapf(err, "couldn't get install %q", installRef.Name)
	}

	appName := fmt.Sprintf("%s-%s", install.Spec.Application, install.Spec.Version)
	var app v1alpha1.Application
	err = rsm.resourceMgr.Get(ctx, appName, installRef.Namespace, &app)
	if err != nil {
		return errors.Wrapf(err, "couldn't get Application %q", appName)
	}

	parameterMgr := NewParameterManager(rsm.kbClient, installRef.Name, app.Spec.ParameterDefinitions, install.Spec.Parameters)
	m, err := parameterMgr.GetMergedMap()
	if err != nil {
		return errors.Wrap(err, "couldn't get merged map")
	}
	resourceNamespace := m["namespace"]
	resourceSuffix := install.Spec.Suffix
	if resourceSuffix != "" {
		resourceSuffix = "-" + resourceSuffix
	}

	for _, resource := range app.Spec.Resources {
		var rsrc resources.DeployableResource
		switch RolloutType(resource.Type) {
		case Deployment:
			rsrc = resources.NewDeployment(rsm.kbClient, resource.Category, installRef.Name, resource.Name+resourceSuffix, resourceNamespace)
		case Statefulset:
			rsrc = resources.NewStatefulSet(rsm.kbClient, resource.Category, installRef.Name, resource.Name+resourceSuffix, resourceNamespace)
		case Job:
			rsrc = resources.NewJob(rsm.kbClient, resource.Category, installRef.Name, resource.Name+resourceSuffix, resourceNamespace)
		case Daemonset:
			rsrc = resources.NewDaemonSet(rsm.kbClient, resource.Category, installRef.Name, resource.Name+resourceSuffix, resourceNamespace)
		case Kubegres:
			rsrc = resources.NewKubegres(rsm.kbClient, resource.Category, installRef.Name, resource.Name+resourceSuffix, resourceNamespace)
		case CronJob:
			rsrc = resources.NewCronJob(rsm.kbClient, resource.Category, installRef.Name, resource.Name+resourceSuffix, resourceNamespace)
		case Service:
			return nil
		default:
			return fmt.Errorf("unrecognized resource rollout type: %q", resource.Type)
		}

		err := rsrc.Wait(timeout)
		if err != nil {
			// TODO: figure out why the wait failed
			return errors.Wrapf(err, "error waiting on resource category %q", resource.Category)
		}
		log.WithFields(log.Fields{"installName": installRef.Name, "installNamespace": installRef.Namespace,
			"resourceName": resource.Name + resourceSuffix, "resourceNamespace": resourceNamespace, "type": resource.Type}).
			Info("Wait successful")
	}
	return nil
}
