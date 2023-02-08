package managers

import (
	"context"
	"embed"
	"path/filepath"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var (
	//go:embed resources/crds/* resources/bindings/* resources/flavors/*
	embeddedResources embed.FS
)

const (
	crdDir     = "resources/crds"
	bindingDir = "resources/bindings"
	flavorDir  = "resources/flavors"
	K0S        = "k0s"
	GKE        = "gke"
)

// BootstrapManager installs the necessary CRDs for kube-bundler to work
type BootstrapManager struct {
	c           KBClient
	resourceMgr *ResourceManager
}

type GCRInfo struct {
	RegistryURL string
	ProjectID   string
}

func NewBootstrapManager(c KBClient) *BootstrapManager {
	return &BootstrapManager{
		c:           c,
		resourceMgr: NewResourceManager(c),
	}
}

func (sm *BootstrapManager) DeployAll(ctx context.Context, skipCRDs, skipBindings, skipFlavors, skipAirgap bool, provider string, gcrInfo GCRInfo) error {
	registryMgr := NewRegistryManager(sm.c)

	if !skipCRDs {
		err := sm.DeployCRDs(ctx)
		if err != nil {
			return errors.Wrap(err, "couldn't deploy CRDs")
		}
	}

	if !skipBindings {
		err := sm.DeployBindings(ctx)
		if err != nil {
			return errors.Wrap(err, "couldn't deploy bindings")
		}
	}

	if !skipFlavors {
		err := sm.DeployFlavors(ctx)
		if err != nil {
			return errors.Wrap(err, "couldn't deploy flavors")
		}
	}

	if !skipAirgap {
		err := registryMgr.DeployProxy(ctx, provider, gcrInfo)
		if err != nil {
			return errors.Wrap(err, "couldn't deploy registry nginx proxy required for airgap support")
		}
	}

	return nil
}

func (sm *BootstrapManager) DeployCRDs(ctx context.Context) error {
	entries, err := embeddedResources.ReadDir(crdDir)
	if err != nil {
		return errors.Wrap(err, "couldn't read embeded files")
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		b, err := embeddedResources.ReadFile(filepath.Join(crdDir, entry.Name()))
		if err != nil {
			return errors.Wrapf(err, "couldn't read embeded file %q", entry.Name())
		}

		m := make(map[string]interface{})
		err = yaml.Unmarshal(b, &m)
		if err != nil {
			return errors.Wrapf(err, "couldn't unmarshal yaml in file '%s'", entry.Name())
		}

		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(m)

		log.WithFields(log.Fields{"name": entry.Name()}).Info("Applying CRD")
		err = sm.resourceMgr.Apply(ctx, u)
		if err != nil {
			return errors.Wrapf(err, "couldn't apply yaml from file '%s'", entry.Name())
		}
	}

	return nil
}

func (sm *BootstrapManager) DeployBindings(ctx context.Context) error {
	entries, err := embeddedResources.ReadDir(bindingDir)
	if err != nil {
		return errors.Wrap(err, "couldn't read embeded files")
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		b, err := embeddedResources.ReadFile(filepath.Join(bindingDir, entry.Name()))
		if err != nil {
			return errors.Wrapf(err, "couldn't read embeded file %q", entry.Name())
		}

		m := make(map[string]interface{})
		err = yaml.Unmarshal(b, &m)
		if err != nil {
			return errors.Wrapf(err, "couldn't unmarshal yaml in file '%s'", entry.Name())
		}

		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(m)

		log.WithFields(log.Fields{"name": entry.Name()}).Info("Applying binding")
		err = sm.resourceMgr.Apply(ctx, u)
		if err != nil {
			return errors.Wrapf(err, "couldn't apply yaml from file '%s'", entry.Name())
		}
	}

	return nil
}

func (sm *BootstrapManager) DeployFlavors(ctx context.Context) error {
	entries, err := embeddedResources.ReadDir(flavorDir)
	if err != nil {
		return errors.Wrap(err, "couldn't read embedded flavor files")
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		b, err := embeddedResources.ReadFile(filepath.Join(flavorDir, entry.Name()))
		if err != nil {
			return errors.Wrapf(err, "couldn't read embedded file %q", entry.Name())
		}

		m := make(map[string]interface{})
		err = yaml.Unmarshal(b, &m)
		if err != nil {
			return errors.Wrapf(err, "couldn't unmarshal yaml in file '%s'", entry.Name())
		}

		u := &unstructured.Unstructured{}
		u.SetUnstructuredContent(m)

		log.WithFields(log.Fields{"name": entry.Name()}).Info("Applying Flavor")
		err = sm.resourceMgr.Apply(ctx, u)
		if err != nil {
			return errors.Wrapf(err, "couldn't apply yaml from file '%s'", entry.Name())
		}
	}

	return nil
}
