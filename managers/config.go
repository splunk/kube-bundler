package managers

import (
	"context"
	"fmt"

	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type ConfigManager struct {
	kbClient    KBClient
	resourceMgr *ResourceManager
}

func NewConfigManager(kbClient KBClient) *ConfigManager {
	return &ConfigManager{
		kbClient:    kbClient,
		resourceMgr: NewResourceManager(kbClient),
	}
}

func (cm *ConfigManager) Get(ctx context.Context, installRef InstallReference, key string) (string, error) {
	// reject unknown configs for a given bundle
	configMap, err := cm.List(ctx, installRef)
	if err != nil {
		return "", errors.Wrap(err, "failed to get config map")
	}
	if _, isPresent := configMap[key]; !isPresent {
		return "", fmt.Errorf("unknown config '%s' for install '%s'", key, installRef.Name)
	}

	var install v1alpha1.Install
	err = cm.resourceMgr.Get(ctx, installRef.Name, installRef.Namespace, &install)
	if err != nil {
		return "", errors.Wrapf(err, "couldn't get install %q", installRef.Name)
	}

	appName := fmt.Sprintf("%s-%s", install.Spec.Application, install.Spec.Version)
	var app v1alpha1.Application
	err = cm.resourceMgr.Get(ctx, appName, installRef.Namespace, &app)
	if err != nil {
		return "", errors.Wrapf(err, "couldn't get application %q", appName)
	}

	pm := NewParameterManager(cm.kbClient, installRef.Name, app.Spec.ParameterDefinitions, install.Spec.Parameters)
	m, err := pm.GetMergedMap()
	if err != nil {
		return "", errors.Wrap(err, "couldn't get merged map")
	}
	return m[key], nil
}

func (cm *ConfigManager) List(ctx context.Context, installRef InstallReference) (map[string]string, error) {
	var install v1alpha1.Install
	err := cm.resourceMgr.Get(ctx, installRef.Name, installRef.Namespace, &install)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't get install %q", installRef.Name)
	}

	appName := fmt.Sprintf("%s-%s", install.Spec.Application, install.Spec.Version)
	var app v1alpha1.Application
	err = cm.resourceMgr.Get(ctx, appName, installRef.Namespace, &app)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't get application %q", appName)
	}

	pm := NewParameterManager(cm.kbClient, installRef.Name, app.Spec.ParameterDefinitions, install.Spec.Parameters)
	m, err := pm.GetMergedMap()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get merged map")
	}
	return m, nil
}

func (cm *ConfigManager) Set(ctx context.Context, installRef InstallReference, key, value string) error {
	// reject unknown configs for a given bundle
	configMap, err := cm.List(ctx, installRef)
	if err != nil {
		return errors.Wrap(err, "failed to get config map")
	}
	if _, isPresent := configMap[key]; !isPresent {
		return fmt.Errorf("unknown config '%s' for install '%s'", key, installRef.Name)
	}

	var originalInstall v1alpha1.Install
	err = cm.resourceMgr.Get(ctx, installRef.Name, installRef.Namespace, &originalInstall)
	if err != nil {
		return errors.Wrapf(err, "couldn't find install %q", installRef.Name)
	}

	newInstall := originalInstall.DeepCopy()

	found := false
	for i, parameter := range newInstall.Spec.Parameters {
		if parameter.Name == key {
			newInstall.Spec.Parameters[i].Value = value
			found = true
			log.WithFields(log.Fields{"name": key, "value": value}).Debug("Overriding existing parameter")
			break
		}
	}

	if !found {
		newInstall.Spec.Parameters = append(newInstall.Spec.Parameters, v1alpha1.ParameterSpec{
			Name:  key,
			Value: value,
		})
		log.WithFields(log.Fields{"name": key, "value": value}).Debug("Adding new parameter")
	}

	err = cm.resourceMgr.Patch(ctx, newInstall, &originalInstall)
	if err != nil {
		return errors.Wrapf(err, "couldn't patch install %q", installRef.Name)
	}

	return nil
}

func (cm *ConfigManager) Remove(ctx context.Context, installRef InstallReference, key string) error {
	var originalInstall v1alpha1.Install
	err := cm.resourceMgr.Get(ctx, installRef.Name, installRef.Namespace, &originalInstall)
	if err != nil {
		return errors.Wrapf(err, "couldn't find install %q", installRef.Name)
	}

	newInstall := originalInstall.DeepCopy()
	newInstall.Spec.Parameters = make([]v1alpha1.ParameterSpec, 0)

	secret, err := getGlobalSecret(cm.kbClient)
	if err != nil {
		return errors.Wrap(err, "Failed to get global secret")
	}

	for i, parameter := range originalInstall.Spec.Parameters {
		if parameter.Name != key {
			newInstall.Spec.Parameters = append(newInstall.Spec.Parameters, originalInstall.Spec.Parameters[i])
		} else {
			// if the parameter uses generateSecret, get the default generated secret from global-secrets
			secretKey := installRef.Name + "." + parameter.Name
			secretValue, found := secret.Data[secretKey]
			if found {
				newInstall.Spec.Parameters = append(newInstall.Spec.Parameters, v1alpha1.ParameterSpec{
					Name:           parameter.Name,
					Value:          string(secretValue[:]),
					GenerateSecret: parameter.GenerateSecret,
				})
			} else {
				log.WithFields(log.Fields{"name": key, "value": parameter.Value}).Debug("Removing parameter from install")
			}
		}
	}

	err = cm.resourceMgr.Patch(ctx, newInstall, &originalInstall)
	if err != nil {
		return errors.Wrapf(err, "couldn't patch install %q", installRef.Name)
	}

	return nil
}
