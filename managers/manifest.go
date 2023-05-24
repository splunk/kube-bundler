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
	"time"

	"github.com/pkg/errors"
	"github.com/quipo/dependencysolver"
	log "github.com/sirupsen/logrus"
	"github.com/splunk/kube-bundler/api/v1alpha1"
)

type ManifestReference struct {
	Name      string
	Namespace string
}

type ManifestManager struct {
	kbClient           KBClient
	resourceMgr        *ResourceManager
	registerMgr        *RegisterManager
	registryMgr        *RegistryManager
	installMgr         *InstallManager
	deployMgr          *DeployManager
	smoketestMgr       *SmoketestManager
	deploySmoketestMgr *DeploySmoketestManager
}

func NewManifestManager(kbClient KBClient) *ManifestManager {
	return &ManifestManager{
		kbClient:           kbClient,
		resourceMgr:        NewResourceManager(kbClient),
		registerMgr:        NewRegisterManager(kbClient),
		registryMgr:        NewRegistryManager(kbClient),
		installMgr:         NewInstallManager(kbClient),
		deployMgr:          NewDeployManager(kbClient),
		smoketestMgr:       NewSmoketestManager(kbClient),
		deploySmoketestMgr: NewDeploySmoketestManager(kbClient),
	}
}

// Install installs all the bundles listed in this manifest
func (mm *ManifestManager) Install(ctx context.Context, manifestRef ManifestReference, force bool) error {
	var manifest v1alpha1.Manifest
	err := mm.resourceMgr.Get(ctx, manifestRef.Name, manifestRef.Namespace, &manifest)
	if err != nil {
		return errors.Wrapf(err, "couldn't get manifest %q", manifestRef.Name)
	}

	// verify all nodes meet minimum CPU/Memory requirements
	err = verifyResourceRequirements(ctx, *mm.resourceMgr, mm.kbClient, manifest)
	if err != nil {
		if force {
			log.Warnf("Forcing installation with insufficient resources for flavor %v", manifest.Spec.Flavor)
		} else {
			return errors.Wrap(err, "Insufficient resources for install")
		}
	}

	// Build the list of BundleRefs used for registration. Since we are iterating over the bundles, also assemble the parameters in a map
	// for ease of lookup
	var bundleRefs []BundleRef
	parameters := make(map[string][]v1alpha1.ParameterSpec)
	additionalParameters := make(map[string][]v1alpha1.ParameterSpec)

	for _, bundle := range manifest.Spec.Bundles {
		parameters[bundle.Name] = bundle.Parameters
		for _, require := range bundle.Requires {
			suffixedResourceName := getResourceName(require.Name, require.Suffix)
			if len(require.Parameters) > 0 {
				additionalParameters[suffixedResourceName] = require.Parameters
			}
		}

		bundleRefs = append(bundleRefs, BundleRef{Name: bundle.Name, Version: bundle.Version})
	}

	var sources []Source
	for _, sourceInfo := range manifest.Spec.Sources {
		var src v1alpha1.Source
		err := mm.resourceMgr.Get(ctx, sourceInfo.Name, manifestRef.Namespace, &src)
		if err != nil {
			return errors.Wrapf(err, "couldn't get source '%s'", sourceInfo.Name)
		}

		newSource, err := NewSource(src.Spec.Type, src.Spec.Path, src.Spec.Options, sourceInfo.Section, sourceInfo.Release)
		if err != nil {
			return errors.Wrap(err, "couldn't create new source")
		}
		sources = append(sources, newSource)
	}
	multiSource := NewMultiSource(sources)

	// Register the bundles
	apps, err := mm.registerMgr.RegisterAll(ctx, bundleRefs, multiSource, manifestRef.Namespace)
	if err != nil {
		return errors.Wrap(err, "couldn't register bundles")
	}

	// Get the registry and its base URL
	dockerRegistry := ""
	if manifest.Spec.Registry != "" {
		var registry v1alpha1.Registry
		err := mm.resourceMgr.Get(ctx, manifest.Spec.Registry, manifestRef.Namespace, &registry)
		if err != nil {
			return errors.Wrapf(err, "couldn't get registry '%s'", manifest.Spec.Registry)
		}
		dockerRegistry = registry.ClusterUrl()
	}

	// Collect the suffixes to apply during Install creation
	suffixes := make(map[string]map[string]string)
	for _, app := range apps {
		for _, require := range app.Spec.Requires {
			_, found := suffixes[require.Name]
			if !found {
				suffixes[require.Name] = make(map[string]string)
			}
			suffixes[require.Name][app.Spec.Name] = require.Suffix
		}
	}

	// Install the installs
	for _, app := range apps {
		pm := NewParameterManager(mm.kbClient, app.Spec.Name, app.Spec.ParameterDefinitions, parameters[app.Spec.Name])
		if suffixes[app.Spec.Name] != nil {
			// Install once per suffix
			for _, suffix := range suffixes[app.Spec.Name] {

				appParameters := pm.parameters
				suffixedResourceName := getResourceName(app.Spec.Name, suffix)
				if len(additionalParameters[suffixedResourceName]) > 0 {
					appParameters = pm.MergeAdditionalParameters(additionalParameters[suffixedResourceName])
				}
				_, err := mm.installMgr.Install(ctx, app.Spec.Name, app.Spec.Name, manifestRef.Namespace, app.Spec.Version, suffix, manifest.Spec.Flavor, dockerRegistry, force, appParameters)
				if err != nil {
					return errors.Wrapf(err, "couldn't install application %s", app.Spec.Name)
				}
			}
		} else {
			_, err := mm.installMgr.Install(ctx, app.Spec.Name, app.Spec.Name, manifestRef.Namespace, app.Spec.Version, "", manifest.Spec.Flavor, dockerRegistry, force, parameters[app.Spec.Name])
			if err != nil {
				return errors.Wrapf(err, "couldn't install application %s", app.Spec.Name)
			}
		}
	}

	return nil
}

func (mm *ManifestManager) Deploy(ctx context.Context, manifestRef ManifestReference, showLogs bool, timeout time.Duration) error {
	smoketest := false
	return mm.deploy(ctx, manifestRef, showLogs, timeout, smoketest)
}

// Deploy deploys all the bundles listed in this manifest
func (mm *ManifestManager) DeploySmoketest(ctx context.Context, manifestRef ManifestReference, showLogs bool, timeout time.Duration) error {
	smoketest := true
	return mm.deploy(ctx, manifestRef, showLogs, timeout, smoketest)
}

func (mm *ManifestManager) deploy(ctx context.Context, manifestRef ManifestReference, showLogs bool, timeout time.Duration, smoketest bool) error {
	var manifest v1alpha1.Manifest
	err := mm.resourceMgr.Get(ctx, manifestRef.Name, manifestRef.Namespace, &manifest)
	if err != nil {
		return errors.Wrapf(err, "couldn't get manifest %q", manifestRef.Name)
	}

	var entries []dependencysolver.Entry
	installMap := make(map[string]*v1alpha1.Install)

	// Collect the suffixes to apply during deploy. The suffixes map describes which dependencies should be deployed with suffixes.
	// For example, as a reusable bundle, postgres might be deployed for multiple services, each of which has a unique suffix.
	suffixes := make(map[string]map[string]bool)
	for _, bundle := range manifest.Spec.Bundles {
		_, found := suffixes[bundle.Name]
		if !found {
			suffixes[bundle.Name] = make(map[string]bool)
		}

		for _, require := range bundle.Requires {
			_, found := suffixes[require.Name]
			if !found {
				suffixes[require.Name] = make(map[string]bool)
			}
			suffixes[require.Name][require.Suffix] = true
			log.WithFields(log.Fields{"bundle": bundle.Name, "requireName": require.Name, "requireSuffix": require.Suffix}).Debug("Adding suffix to map")
		}
	}
	log.WithFields(log.Fields{"suffixes": suffixes}).Debug("Assembled suffix map")

	// Collect the dependencies to apply during deploy. The dependencies map includes the suffix, if applicable.
	dependencies := make(map[string]map[string]bool)
	for _, bundle := range manifest.Spec.Bundles {
		for _, require := range bundle.Requires {
			_, found := dependencies[bundle.Name]
			if !found {
				dependencies[bundle.Name] = make(map[string]bool)
			}
			requireName := require.Name
			if require.Suffix != "" {
				requireName += "-" + require.Suffix
			}
			dependencies[bundle.Name][requireName] = true
			log.WithFields(log.Fields{"bundle": bundle.Name, "require": requireName}).Debug("Adding require to dependency map")
		}
	}
	log.WithFields(log.Fields{"dependencies": dependencies}).Debug("Assembled dependency map")

	for _, bundle := range manifest.Spec.Bundles {
		// Insert an empty suffix if no suffixes were recorded, to simplify the range on suffixes[bundle.Name]
		if len(suffixes[bundle.Name]) == 0 {
			suffixes[bundle.Name][""] = true
		}

		for suffix := range suffixes[bundle.Name] {
			var install v1alpha1.Install
			installName := bundle.Name
			if suffix != "" {
				installName += "-" + suffix
			}
			err := mm.resourceMgr.Get(ctx, installName, manifestRef.Namespace, &install)
			if err != nil {
				return errors.Wrapf(err, "couldn't get install for bundle '%s'", bundle.Name)
			}
			installMap[installName] = &install

			var app v1alpha1.Application
			appName := fmt.Sprintf("%s-%s", install.Spec.Application, install.Spec.Version)

			err = mm.resourceMgr.Get(ctx, appName, manifestRef.Namespace, &app)
			if err != nil {
				return errors.Wrapf(err, "couldn't get application for bundle '%s'", bundle.Name)
			}

			var deps []string
			for _, requirement := range app.Spec.Requires {
				depSuffixes, found := dependencies[requirement.Name]
				if found {
					for depSuffix := range depSuffixes {
						log.WithFields(log.Fields{"require": requirement.Name, "suffix": depSuffix}).Debug("Adding dependency with suffix for solver")
						deps = append(deps, depSuffix)
					}
				}
				requireNameSuffix := requirement.Name
				if requirement.Suffix != "" {
					requireNameSuffix += "-" + requirement.Suffix
				}
				log.WithFields(log.Fields{"require": requireNameSuffix, "bundle": bundle.Name}).Debug("Adding dependency for solver")
				deps = append(deps, requireNameSuffix)
			}
			entry := dependencysolver.Entry{
				ID:   installName,
				Deps: deps,
			}
			log.WithFields(log.Fields{"entry": entry}).Debug("Adding solver entry")
			entries = append(entries, entry)
		}
	}
	log.WithFields(log.Fields{"entries": entries}).Debug("Assembled final solver entries")

	// Resolve the dependency order
	layers := dependencysolver.LayeredTopologicalSort(entries)
	if layers == nil {
		return errors.New("can't resolve dependencies; may be circular or have missing relationships")
	}

	// Process each layer in the determined order
	for i, layer := range layers {
		log.WithFields(log.Fields{"level": i, "layer": layer}).Info("Processing layer")
		for _, name := range layer {
			install := installMap[name]

			installRef := InstallReference{Name: install.Name, Namespace: install.Namespace}
			if smoketest {
				err := mm.deploySmoketestMgr.DeploySmoketest(ctx, installRef, showLogs, timeout)
				if err != nil {
					return errors.Wrapf(err, "couldn't deploy '%s'", install.Name)
				}
			} else {
				deployOpts := DeployOpts{
					Action:  ActionApplyOutputs,
					Timeout: timeout,
				}
				err := mm.deployMgr.Deploy(ctx, installRef, deployOpts, showLogs)
				if err != nil {
					return errors.Wrapf(err, "couldn't deploy '%s'", install.Name)
				}
			}
		}
	}

	return nil
}

func (mm *ManifestManager) Diff(ctx context.Context, manifestRef ManifestReference, timeout time.Duration) error {
	var manifest v1alpha1.Manifest
	err := mm.resourceMgr.Get(ctx, manifestRef.Name, manifestRef.Namespace, &manifest)
	if err != nil {
		return errors.Wrapf(err, "couldn't get manifest %q", manifestRef.Name)
	}

	for _, bundle := range manifest.Spec.Bundles {
		installRef := InstallReference{Name: bundle.Name, Namespace: manifestRef.Namespace}
		deployOpts := DeployOpts{
			Action:  ActionDiff,
			Timeout: timeout,
		}

		err := mm.deployMgr.Deploy(ctx, installRef, deployOpts, true)
		if err != nil {
			return errors.Wrapf(err, "couldn't diff '%s'", bundle.Name)
		}
	}

	return nil
}

// Smoketest smoketests all the bundles listed in this manifest
func (mm *ManifestManager) Smoketest(ctx context.Context, manifestRef ManifestReference, showLogs bool, timeout time.Duration) error {
	var manifest v1alpha1.Manifest
	err := mm.resourceMgr.Get(ctx, manifestRef.Name, manifestRef.Namespace, &manifest)
	if err != nil {
		return errors.Wrapf(err, "couldn't get manifest %q", manifestRef.Name)
	}

	for _, bundle := range manifest.Spec.Bundles {
		installRef := InstallReference{Name: bundle.Name, Namespace: manifestRef.Namespace}
		err := mm.smoketestMgr.Smoketest(ctx, installRef, showLogs, timeout)
		if err != nil {
			return errors.Wrapf(err, "couldn't deploy '%s'", bundle.Name)
		}
	}

	return nil
}
