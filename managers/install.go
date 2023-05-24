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

package managers

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/splunk/kube-bundler/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type InstallManager struct {
	kbClient    KBClient
	resourceMgr *ResourceManager
}

type InstallDescription struct {
	Name        string
	Application string
	Version     string
	Parameters  map[string]ParameterDesc
}

func NewInstallManager(kbClient KBClient) *InstallManager {
	return &InstallManager{
		kbClient:    kbClient,
		resourceMgr: NewResourceManager(kbClient),
	}
}

// Install creates an install resource if it does not already exist. Subsequent calls to Install do not modify the Install with the provided parameters argument.
// In that case, the provided parameters are also not set on the return Install.
func (im *InstallManager) Install(ctx context.Context, appName, name string, namespace string, version string, suffix string, flavor string, dockerRegistry string, force bool, parameters []v1alpha1.ParameterSpec) (*v1alpha1.Install, error) {
	err := verifyNodeRequirements(ctx, *im.resourceMgr, im.kbClient, flavor, namespace)
	if err != nil {
		if force {
			log.Warnf("Forcing installation with insufficient nodes for flavor %v", flavor)
		} else {
			return nil, errors.Wrap(err, "Insufficient nodes for install")
		}
	}
	var install v1alpha1.Install
	installName := appName
	if name != "" {
		installName = name
	}
	if suffix != "" {
		installName = installName + "-" + suffix
	}

	var installs v1alpha1.InstallList
	opts := client.MatchingFields{"metadata.name": installName, "metadata.namespace": namespace}
	err = im.resourceMgr.List(ctx, namespace, &installs, opts)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't list existing installs")
	}

	foundExisting := false
	if len(installs.Items) == 1 {
		foundExisting = true
		log.WithFields(log.Fields{"name": installName, "namespace": namespace}).Info("Found install, preserving existing parameters")
	}

	install.Kind = "Install"
	install.APIVersion = path.Join(v1alpha1.GroupVersion.Group, v1alpha1.GroupVersion.Version)

	install.Name = installName
	install.Namespace = namespace
	install.Spec.Application = appName
	install.Spec.Version = version
	install.Spec.Suffix = suffix
	install.Spec.Flavor = flavor
	install.Spec.DockerRegistry = dockerRegistry

	if !foundExisting {
		install.Spec.Parameters = parameters
	} else {
		install.Spec.Parameters = installs.Items[0].Spec.Parameters
	}

	// Convert the install to map[string]interface{} for use with server-side apply

	b, err := json.Marshal(&install)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't encode json for install")
	}

	m := make(map[string]interface{})
	err = json.Unmarshal(b, &m)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't decode json for install")
	}

	u := unstructured.Unstructured{}
	u.SetUnstructuredContent(m)

	err = im.resourceMgr.Apply(ctx, &u)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't apply install")

	}

	return &install, nil
}

func (im *InstallManager) Describe(ctx context.Context, installs []string) ([]InstallDescription, error) {
	var list v1alpha1.InstallList
	if len(installs) == 0 {
		err := im.resourceMgr.List(ctx, defaultNamespace, &list)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't list installs")
		}
	} else {
		var install v1alpha1.Install
		err := im.resourceMgr.Get(ctx, installs[0], defaultNamespace, &install)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't get installs")
		}
		list.Items = append(list.Items, install)
	}

	descriptions := make([]InstallDescription, 0)
	for _, install := range list.Items {
		appName := fmt.Sprintf("%s-%s", install.Spec.Application, install.Spec.Version)
		var app v1alpha1.Application
		err := im.resourceMgr.Get(ctx, appName, defaultNamespace, &app)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't get application")
		}

		pm := NewParameterManager(im.kbClient, install.Name, app.Spec.ParameterDefinitions, install.Spec.Parameters)
		params := make(map[string]ParameterDesc)
		for name, parameterDesc := range pm.GetParameterDesc() {
			params[name] = ParameterDesc{
				Value:       parameterDesc.Value,
				Description: parameterDesc.Description,
				Default:     parameterDesc.Default,
			}
		}

		descriptions = append(
			descriptions,
			InstallDescription{
				Name:        install.Name,
				Application: install.Spec.Application,
				Version:     install.Spec.Version,
				Parameters:  params,
			})
	}

	return descriptions, nil
}
