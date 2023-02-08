package managers

import (
	"context"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ResourceManager struct {
	c client.Client
}

func NewResourceManager(c client.Client) *ResourceManager {
	return &ResourceManager{
		c: c,
	}
}

func (m *ResourceManager) Get(ctx context.Context, name string, namespace string, obj client.Object) error {
	err := m.c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj)
	if err != nil {
		return errors.Wrapf(err, "couldn't get resource %q", name)
	}
	log.WithFields(log.Fields{"name": name, "namespace": namespace, "kind": obj.GetObjectKind()}).Debug("Fetch resource")
	return nil
}

func (m *ResourceManager) List(ctx context.Context, namespace string, list client.ObjectList, opts ...client.ListOption) error {
	opts = append(opts, &client.ListOptions{Namespace: namespace})
	err := m.c.List(ctx, list, opts...)
	if err != nil {
		return errors.Wrapf(err, "couldn't list resources in namespace %q", namespace)
	}

	log.WithFields(log.Fields{"namespace": namespace, "kind": list.GetObjectKind()}).Debug("List resource")
	return nil
}

func (m *ResourceManager) Create(ctx context.Context, obj client.Object) error {
	err := m.c.Create(ctx, obj)
	if err != nil {
		return errors.Wrapf(err, "couldn't create resource %q", obj.GetName())
	}
	log.WithFields(log.Fields{"name": obj.GetName(), "namespace": obj.GetNamespace(), "kind": obj.GetObjectKind()}).Debug("Create resource")
	return nil
}

func (m *ResourceManager) CreateIfNotExists(ctx context.Context, obj client.Object) error {
	err := m.c.Get(ctx, client.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}, obj)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			createErr := m.c.Create(ctx, obj)
			if createErr != nil {
				return errors.Wrapf(createErr, "couldn't create resource %q", obj.GetName())
			}
			log.WithFields(log.Fields{"name": obj.GetName(), "namespace": obj.GetNamespace(), "kind": obj.GetObjectKind()}).Debug("Create resource")
			return nil
		}
		return errors.Wrapf(err, "couldn't get resource %q", obj.GetName())
	}

	log.WithFields(log.Fields{"name": obj.GetName(), "namespace": obj.GetNamespace(), "kind": obj.GetObjectKind()}).Debug("Resource already exists")
	return nil
}

func (m *ResourceManager) CreateOrPatch(ctx context.Context, name string, namespace string, obj client.Object, f controllerutil.MutateFn) error {
	err := m.c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			obj.SetName(name)
			obj.SetNamespace(namespace)
		} else {
			return errors.Wrap(err, "couldn't get resource to create or patch")
		}
	}

	_, err = controllerutil.CreateOrPatch(ctx, m.c, obj, f)
	if err != nil {
		return errors.Wrap(err, "couldn't create or update resource")
	}

	return nil
}

func (m *ResourceManager) Patch(ctx context.Context, newObj client.Object, original client.Object) error {
	patch := client.MergeFrom(original)
	err := m.c.Patch(ctx, newObj, patch)
	if err != nil {
		return errors.Wrapf(err, "couldn't patch resource %q", newObj.GetName())
	}
	log.WithFields(log.Fields{"name": newObj.GetName(), "namespace": newObj.GetNamespace(), "kind": newObj.GetObjectKind()}).Debug("Patch resource")
	return nil
}

// Apply applies resources using server-side apply. Similar to `kubectl apply -f`
func (m *ResourceManager) Apply(ctx context.Context, obj client.Object) error {
	opts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("kb")}
	err := m.c.Patch(ctx, obj, client.Apply, opts...)
	if err != nil {
		return errors.Wrapf(err, "couldn't apply resource %q", obj.GetName())
	}
	log.WithFields(log.Fields{"name": obj.GetName(), "namespace": obj.GetNamespace(), "kind": obj.GetObjectKind()}).Debug("Apply resource")
	return nil
}

func (m *ResourceManager) Delete(ctx context.Context, name string, namespace string, obj client.Object) error {
	deleteOpts := []client.DeleteAllOfOption{
		client.InNamespace(namespace),
		client.MatchingFields{"metadata.name": name},
		client.PropagationPolicy("Foreground"),
	}
	err := m.c.DeleteAllOf(ctx, obj, deleteOpts...)
	if client.IgnoreNotFound(err) != nil {
		return errors.Wrap(err, "error deleting resource")
	}

	// Foreground propagation sometimes leaves the object in pending deletion status, so poll the object to ensure it has been deleted
	pollDuration := 1 * time.Second
	for {
		err = m.c.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, obj)
		if client.IgnoreNotFound(err) == nil {
			log.WithFields(log.Fields{"name": name, "namespace": namespace}).Debug("Poll resource condition met")
			break
		} else if err != nil {
			log.WithFields(log.Fields{"name": name, "namespace": namespace, "err": err}).Debug("Poll resource condition encountered error")
			return errors.Wrap(err, "error polling resource")
		}
		log.WithFields(log.Fields{"name": name, "namespace": namespace}).Infof("Poll resource condition not met, waiting %v", pollDuration)
		time.Sleep(pollDuration)
	}

	time.Sleep(pollDuration)
	log.WithFields(log.Fields{"name": name, "namespace": namespace, "kind": obj.GetObjectKind()}).Debug("Delete resource")
	return nil
}
