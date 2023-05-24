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
	"fmt"

	"github.com/pkg/errors"
	"github.com/quipo/dependencysolver"
	log "github.com/sirupsen/logrus"
	"github.com/splunk/kube-bundler/api/v1alpha1"
)

type BundleSource struct {
	Type    string
	Path    string
	Options map[string]string
}

type RegisterManager struct {
	kbClient    KBClient
	resourceMgr *ResourceManager
}

func NewRegisterManager(kbClient KBClient) *RegisterManager {
	return &RegisterManager{
		kbClient:    kbClient,
		resourceMgr: NewResourceManager(kbClient),
	}
}

// Register registers a single bundle. Will fail if required dependencies are not already present
func (rm *RegisterManager) Register(ctx context.Context, bundleRef BundleRef, bundleSource Source, namespace string) (*v1alpha1.Application, error) {
	bundleFile, err := bundleSource.Get(bundleRef)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't find bundle file with name '%s'", bundleRef.Filename())
	}
	defer bundleFile.Close()

	app, err := bundleFile.Application(namespace)
	if err != nil {
		return nil, err
	}
	apps := []*v1alpha1.Application{app}
	err = rm.validateDependencies(ctx, namespace, apps)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't satisfy dependencies")
	}
	return rm.register(ctx, app, bundleFile, namespace)
}

// register registers an application. It assumes all validation has already been performed
func (rm *RegisterManager) register(ctx context.Context, app *v1alpha1.Application, bundleFile *BundleFile, namespace string) (*v1alpha1.Application, error) {
	err := rm.resourceMgr.CreateIfNotExists(ctx, app)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create application definition")
	}
	log.WithFields(log.Fields{"application": app.Name, "bundleFile": bundleFile}).Info("Application registered")

	return app, nil
}

// RegisterAll registers a list of bundles. Will resolve dependency order if all dependencies are included. Returns the installed apps in dependency order
func (rm *RegisterManager) RegisterAll(ctx context.Context, bundleRef []BundleRef, bundleSource Source, namespace string) ([]*v1alpha1.Application, error) {
	// Read all bundles to build the dependency tree
	var entries []dependencysolver.Entry
	bundleFileMap := make(map[string]*BundleFile, len(bundleRef))
	appMap := make(map[string]*v1alpha1.Application, len(bundleRef))

	var apps []*v1alpha1.Application
	for _, filename := range bundleRef {
		bundleFile, err := bundleSource.Get(filename)
		if err != nil {
			return nil, errors.Wrapf(err, "couldn't find bundle file with name '%s'", filename)
		}

		app, err := bundleFile.Application(namespace)
		if err != nil {
			return nil, err
		}
		bundleFileMap[app.Spec.Name] = bundleFile
		appMap[app.Spec.Name] = app
		apps = append(apps, app)

		var deps []string
		for _, requirement := range app.Spec.Requires {
			deps = append(deps, requirement.Name)
		}
		entry := dependencysolver.Entry{
			ID:   app.Spec.Provides[0].Name, // TODO: support more than 1 provides
			Deps: deps,
		}
		entries = append(entries, entry)
	}

	err := rm.validateDependencies(ctx, namespace, apps)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't satisfy dependencies")
	}

	layers := dependencysolver.LayeredTopologicalSort(entries)
	if layers == nil {
		return nil, errors.New("can't resolve circular dependencies")
	}

	// registrationOrderedApps returns the dependency order as determined above, which is likely different than the user-provided bundles
	var registrationOrderedApps []*v1alpha1.Application

	// Process each layer in the determined order
	for i, layer := range layers {
		log.WithFields(log.Fields{"level": i, "layer": layer}).Info("Processing layer")
		for _, name := range layer {
			bundleFile := bundleFileMap[name]
			app := appMap[name]
			registrationOrderedApps = append(registrationOrderedApps, app)

			_, err := rm.register(ctx, app, bundleFile, namespace)
			if err != nil {
				return nil, errors.Wrapf(err, "couldn't register application %q", name)
			}

			_ = bundleFile.Close()
		}
	}

	return registrationOrderedApps, nil
}

func (rm *RegisterManager) validateDependencies(ctx context.Context, namespace string, apps []*v1alpha1.Application) error {
	var installedAppList v1alpha1.ApplicationList
	err := rm.resourceMgr.List(ctx, namespace, &installedAppList)
	if err != nil {
		return errors.Wrap(err, "couldn't list applications")
	}

	var installedAppNames []string
	for _, installedApp := range installedAppList.Items {
		installedAppNames = append(installedAppNames, installedApp.Spec.Name)
	}

	var toBeInstalledApps []string
	for _, app := range apps {
		toBeInstalledApps = append(toBeInstalledApps, app.Spec.Name)
	}

	for _, app := range apps {
		for _, requirement := range app.Spec.Requires {
			if !stringSliceContains(installedAppNames, requirement.Name) && !stringSliceContains(toBeInstalledApps, requirement.Name) {
				return fmt.Errorf("required dependency %q for app %q not found", requirement.Name, app.Name)
			}
			if requirement.Name == app.Spec.Name {
				return fmt.Errorf("bundle %q cannot require itself", requirement.Name)
			}
		}
	}

	return nil
}
