package managers

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"embed"
	_ "embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/avast/retry-go"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// podRegistryDir is the path where the hostpath directory is mounted in the container
	podRegistryDir = "/var/lib/registry"

	// dockerDir is the directory that contains all the docker images
	dockerDir = "docker"

	// defaultHostPathBase is the base directory used to store the registry contents on the host
	defaultHostPathBase = "/var/lib/registry"
	imagePath           = "REGISTRY_URL/PROJECT_ID/IMAGE"
)

var (
	//go:embed yaml/*
	files embed.FS
)

//go:embed yaml/registry-nginx-proxy-configmap.yaml
var registryProxyConfigYaml string

//go:embed yaml/registry-nginx-proxy-daemonset.yaml
var registryProxyDaemonsetYaml string

type RegistryRef struct {
	Name      string
	Namespace string
}

type RegistryManager struct {
	c           KBClient
	resourceMgr *ResourceManager
}

func NewRegistryManager(c KBClient) *RegistryManager {
	return &RegistryManager{
		c:           c,
		resourceMgr: NewResourceManager(c),
	}
}

type registryArgs struct {
	RegistryName string
	Image        string
	Replicas     int
	NodeSelector map[string]string
	HostPath     string
}

// Deploy creates the registry deployment
func (rm *RegistryManager) Deploy(ctx context.Context, registryRef RegistryRef) error {
	var registry v1alpha1.Registry
	err := rm.resourceMgr.Get(ctx, registryRef.Name, registryRef.Namespace, &registry)
	if err != nil {
		return errors.Wrapf(err, "couldn't get registry %q", registryRef.Name)
	}

	flavorName := registry.Spec.Flavor
	if flavorName == "" {
		flavorName = DefaultResourceName
	}

	var flavor v1alpha1.Flavor
	err = rm.resourceMgr.Get(ctx, flavorName, defaultNamespace, &flavor)
	if err != nil {
		return errors.Wrapf(err, "couldn't get flavor %q", flavorName)
	}

	hostPath := defaultHostPathBase
	if strings.TrimSpace(registry.Spec.HostPath) != "" {
		hostPath = registry.Spec.HostPath
	}

	r := registryArgs{
		RegistryName: registry.SanitizedRegistryName(),
		Image:        registry.Spec.Image,
		Replicas:     flavor.Spec.StatefulQuorumReplicas,
		NodeSelector: registry.Spec.NodeSelector,
		HostPath:     hostPath,
	}

	templatesDir := "yaml/deployment"
	tmpl, err := template.ParseFS(files, filepath.Join(templatesDir, "*"))
	if err != nil {
		return errors.Wrap(err, "couldn't parse templates")
	}

	tmplFiles, err := fs.ReadDir(files, templatesDir)
	if err != nil {
		return errors.Wrap(err, "couldn't read embedded templates directory")
	}

	var buf bytes.Buffer
	for _, file := range tmplFiles {
		if file.IsDir() {
			continue
		}

		err = tmpl.ExecuteTemplate(&buf, file.Name(), r)
		if err != nil {
			return errors.Wrapf(err, "couldn't execute template '%s'", file.Name())
		}

		resource, err := rm.loadYaml(buf.String())
		if err != nil {
			return errors.Wrapf(err, "couldn't load resource '%s'", file.Name())
		}

		log.WithFields(log.Fields{"file": file.Name()}).Info("Applying resource")
		err = rm.applyUnstructured(ctx, resource)
		if err != nil {
			return errors.Wrapf(err, "couldn't apply resource from '%s'", file.Name())
		}

		buf.Reset()
	}

	// Wait on registry deployment
	deployName := fmt.Sprintf("registry-%s", r.RegistryName)
	err = rm.WaitForRunning(ctx, deployName)
	if err != nil {
		return errors.Wrapf(err, "error waiting on registry Deployment resource %q", deployName)
	}

	return nil
}

// Delete deletes the registry referred to by registryRef and its associated resources
func (rm *RegistryManager) Delete(ctx context.Context, registryRef RegistryRef) error {
	var registry v1alpha1.Registry
	err := rm.resourceMgr.Get(ctx, registryRef.Name, registryRef.Namespace, &registry)
	if err != nil {
		return errors.Wrapf(err, "couldn't get registry %q", registryRef.Name)
	}

	resourceName := "registry-" + registry.SanitizedRegistryName()

	var pods corev1.PodList
	opts := client.MatchingLabels{"name": resourceName}
	err = rm.resourceMgr.List(ctx, registryRef.Namespace, &pods, opts)
	if err != nil {
		return errors.Wrap(err, "couldn't list pods of deployment")
	}

	for _, pod := range pods.Items {
		// Delete the registry contents. Although we could delete podRegistryDir, that would delete
		// the hostpath mount as well. Specifying the docker dir means we don't have to supply a wildcard
		// like podRegistryDir/* to rm -rf
		log.WithFields(log.Fields{"pod": pod.Name}).Info("Deleting registry contents")
		err = rm.deleteRegistryContents(pod, filepath.Join(podRegistryDir, dockerDir))
		if err != nil {
			return errors.Wrapf(err, "couldn't delete registry contents in pod %q", pod.Name)
		}
	}

	var deploy appsv1.Deployment
	err = rm.resourceMgr.Delete(ctx, resourceName, registryRef.Namespace, &deploy)
	if err != nil {
		return errors.Wrapf(err, "couldn't delete deployment for registry %q", registryRef.Name)
	}

	var svc corev1.Service
	err = rm.resourceMgr.Delete(ctx, resourceName, registryRef.Namespace, &svc)
	if err != nil {
		return errors.Wrapf(err, "couldn't delete service for registry %q", registryRef.Name)
	}

	err = rm.resourceMgr.Delete(ctx, registryRef.Name, registryRef.Namespace, &registry)
	if err != nil {
		return errors.Wrapf(err, "couldn't delete registry %q", registryRef.Name)
	}

	return nil
}

// DeployProxy creates the nginx proxy daemonset for the registries
func (rm *RegistryManager) DeployProxy(ctx context.Context, provider string, gcrInfo GCRInfo) error {

	// port number on the node where the proxy will listen on the cluster (across all nodes)
	port := "6000"
	// Namespace where the registry is deployed. This is needed by kube dns resolver when resolving registry svc name
	registryNamespace := "default"
	registryProxyConfig := registryProxyConfigYaml
	registryProxyConfig = strings.ReplaceAll(registryProxyConfig, "__PORT__", port)
	registryProxyConfig = strings.ReplaceAll(registryProxyConfig, "__REGISTRY_NAMESPACE__", registryNamespace)

	err := rm.applyYaml(ctx, registryProxyConfig)
	if err != nil {
		return errors.Wrap(err, "couldn't apply registry nginx proxy configmap")
	}

	registryProxyDaemonset := registryProxyDaemonsetYaml
	registryProxyDaemonset = strings.ReplaceAll(registryProxyDaemonset, "__PORT__", port)

	if provider == GKE {
		//build image url
		//Image existence validation
		url := buildGCRImageURL(gcrInfo)
		registryProxyDaemonset = strings.ReplaceAll(registryProxyDaemonset, "nginx:1.22.0-alpine-11", url)
		registryProxyDaemonset = strings.ReplaceAll(registryProxyDaemonset, "imagePullPolicy: Never", "")
	}

	err = rm.applyYaml(ctx, registryProxyDaemonset)
	if err != nil {
		return errors.Wrap(err, "couldn't apply registry nginx proxy daemonset")
	}

	return nil
}

func buildGCRImageURL(gcrInfo GCRInfo) string {
	var replacers = strings.NewReplacer("REGISTRY_URL", gcrInfo.RegistryURL, "PROJECT_ID", gcrInfo.ProjectID, "IMAGE", "nginx:1.22.0-alpine-11")
	return replacers.Replace(imagePath)
}

func (rm *RegistryManager) loadYaml(yamlText string) (map[string]interface{}, error) {
	yamlMap := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(yamlText), &yamlMap)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't read yaml")
	}
	return yamlMap, nil
}

func (rm *RegistryManager) applyYaml(ctx context.Context, yamlText string) error {
	yamlMap, err := rm.loadYaml(yamlText)
	if err != nil {
		return err
	}
	return rm.applyUnstructured(ctx, yamlMap)
}

func (rm *RegistryManager) applyUnstructured(ctx context.Context, yamlMap map[string]interface{}) error {
	u := &unstructured.Unstructured{}
	u.SetUnstructuredContent(yamlMap)

	err := rm.resourceMgr.Apply(ctx, u)
	if err != nil {
		return errors.Wrap(err, "couldn't apply yaml")
	}

	return nil
}

// Import imports the airgap bundles into the pod. If destDir is non-empty, the contents are copied directly to that local path on the host filesystem.
func (rm *RegistryManager) Import(ctx context.Context, registryRef RegistryRef, source Source, bundles []BundleRef, destDir, hostArg string) error {
	var registry v1alpha1.Registry
	err := rm.resourceMgr.Get(ctx, registryRef.Name, registryRef.Namespace, &registry)
	if err != nil {
		return errors.Wrapf(err, "couldn't get registry %q", registryRef.Name)
	}

	deploymentName := "registry-" + registry.SanitizedRegistryName()
	var pods corev1.PodList
	opts := client.MatchingLabels{"name": deploymentName}
	err = rm.resourceMgr.List(ctx, registryRef.Namespace, &pods, opts)
	if err != nil {
		return errors.Wrap(err, "couldn't list pods of deployment")
	}

	var filteredPods []corev1.Pod
	if hostArg != "" {
		for _, pod := range pods.Items {
			if pod.Status.HostIP == hostArg {
				filteredPods = append(filteredPods, pod)
				break
			}
		}
	} else {
		filteredPods = append(filteredPods, pods.Items...)
	}

	for _, pod := range filteredPods {
		err = rm.ImportToPod(ctx, pod, registryRef, source, bundles, destDir)
		if err != nil {
			return errors.Wrapf(err, "couldn't import images to pod %s", pod.Name)
		}
	}

	return nil
}

func (rm *RegistryManager) ImportToPod(ctx context.Context, pod corev1.Pod, registryRef RegistryRef, source Source, bundles []BundleRef, destDir string) error {
	for _, bundleRef := range bundles {
		bundleFile, err := source.Get(bundleRef)
		if err != nil {
			return errors.Wrapf(err, "couldn't get bundle file '%s' from source", bundleRef.Name)
		}

		zipReader, err := zip.NewReader(bundleFile.Contents, bundleFile.Size)
		if err != nil {
			return errors.Wrapf(err, "couldn't open zip reader to bundle file '%s'", bundleRef.Name)
		}

		tarContents, err := zipReader.Open(DefaultImagesFile)
		if err != nil && errors.Is(err, fs.ErrNotExist) {
			log.WithFields(log.Fields{"bundle": bundleRef.Name}).Info("No images found for bundle, skipping")
			continue
		} else if err != nil {
			return errors.Wrapf(err, "couldn't open images.tar from bundle file '%s'", bundleRef.Name)
		}

		fi, err := tarContents.Stat()
		if err != nil {
			return errors.Wrapf(err, "couldn't stat images.tar from bundle file '%s'", bundleRef.Name)
		}
		if fi.Size() == 0 {
			log.WithFields(log.Fields{"bundle": bundleRef.Name}).Info("Empty images.tar found for bundle, skipping")
			tarContents.Close()
			continue
		}

		log.WithFields(log.Fields{"bundle": bundleRef.Name}).Info("Importing images for bundle")
		if destDir != "" {
			// Copy directly to hostpath directory
			hostpath := filepath.Join(destDir, registryRef.Name)
			log.WithField("hostpath", hostpath).Info("importing to directory")
			err = rm.extractTarToDir(hostpath, tarContents)
			if err != nil {
				tarContents.Close()
				return errors.Wrapf(err, "couldn't copy images.tar to registry for bundle file '%s'", bundleRef.Name)
			}
		} else {
			// Copy via pod exec
			err = rm.extractTarToPod(pod, podRegistryDir, tarContents)
			if err != nil {
				tarContents.Close()
				return errors.Wrapf(err, "couldn't copy images.tar to registry for bundle file '%s'", bundleRef.Name)
			}
		}

		tarContents.Close()
	}

	return nil
}

func (rm *RegistryManager) ImportManifest(ctx context.Context, manifestRef ManifestReference, destDir, hostArg string) error {
	var manifest v1alpha1.Manifest
	err := rm.resourceMgr.Get(ctx, manifestRef.Name, manifestRef.Namespace, &manifest)
	if err != nil {
		return errors.Wrapf(err, "couldn't get manifest %q", manifestRef.Name)
	}

	var sources []Source
	for _, sourceInfo := range manifest.Spec.Sources {
		var src v1alpha1.Source
		err := rm.resourceMgr.Get(ctx, sourceInfo.Name, manifestRef.Namespace, &src)
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

	var bundleRefs []BundleRef
	for _, bundle := range manifest.Spec.Bundles {
		bundleRefs = append(bundleRefs, BundleRef{Name: bundle.Name, Version: bundle.Version})
	}

	registryRef := RegistryRef{Name: manifest.Spec.Registry, Namespace: manifestRef.Namespace}
	err = rm.Import(ctx, registryRef, multiSource, bundleRefs, destDir, hostArg)
	if err != nil {
		return errors.Wrap(err, "couldn't import bundles from manifest")
	}

	return nil
}

// deleteRegistryContents deletes all the files that were imported via Import(). Since this does a pod exec, the
// `rm -rf` command will be limited in scope to the pod filesystem
func (rm *RegistryManager) deleteRegistryContents(pod corev1.Pod, dstDir string) error {
	clientset := rm.c.Interface

	command := fmt.Sprintf("rm -rf '%s'", dstDir)
	request := clientset.CoreV1().RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"/bin/sh", "-c", command},
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
		}, runtime.NewParameterCodec(rm.c.Scheme()))

	exec, err := remotecommand.NewSPDYExecutor(rm.c.RestConfig, "POST", request.URL())
	if err != nil {
		return errors.Wrap(err, "couldn't create spdy executor")
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})

	if err != nil {
		return errors.Wrapf(err, "couldn't delete files from '%s'", dstDir)
	}

	return nil
}

func (rm *RegistryManager) extractTarToPod(pod corev1.Pod, dstDir string, tarContents io.Reader) error {
	clientset := rm.c.Interface

	command := fmt.Sprintf("tar x -C %s -f -", dstDir)
	request := clientset.CoreV1().RESTClient().
		Post().
		Namespace(pod.Namespace).
		Resource("pods").
		Name(pod.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"/bin/sh", "-c", command},
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
		}, runtime.NewParameterCodec(rm.c.Scheme()))

	exec, err := remotecommand.NewSPDYExecutor(rm.c.RestConfig, "POST", request.URL())
	if err != nil {
		return errors.Wrap(err, "couldn't create spdy executor")
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  tarContents,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})

	if err != nil {
		return errors.Wrapf(err, "couldn't extract files to '%s'", dstDir)
	}

	return nil
}

func (rm *RegistryManager) extractTarToDir(dstDir string, tarContents io.Reader) error {
	tarReader := tar.NewReader(tarContents)
	var header *tar.Header
	var err error

	for {
		header, err = tarReader.Next()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return errors.Wrap(err, "couldn't extract tar")
		}

		fullName := filepath.Join(dstDir, header.Name)
		log.WithFields(log.Fields{"name": fullName}).Debug("Processing tar entry")
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(fullName, 0755); err != nil {
				return errors.Wrapf(err, "couldn't create dir '%s'", fullName)
			}
		case tar.TypeReg:
			fi, err := os.Stat(fullName)
			if err != nil && !os.IsNotExist(err) {
				return errors.Wrapf(err, "couldn't stat existing file '%s'", fullName)
			}

			out, err := os.OpenFile(fullName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_EXCL, 0644)
			if err != nil {
				if os.IsExist(err) && fi.Size() == header.Size && fi.ModTime() == header.ModTime {
					// Skip file
					log.WithField("file", fullName).Debug("Skipping existing file with both matching size and modify time")
					continue
				} else if os.IsExist(err) && fi.Size() == header.Size {
					// Skip file
					log.WithField("file", fullName).Debug("Skipping existing file with matching size only")
					continue
				} else if !os.IsExist(err) {
					// If the file already exists, continue below and overwrite it. Otherwise return the error
					return errors.Wrapf(err, "couldn't create file '%s'", fullName)
				}
			}

			_, err = io.Copy(out, tarReader)
			if err != nil {
				return errors.Wrapf(err, "couldn't copy file '%s' to destination", header.Name)
			}

			err = out.Close()
			if err != nil {
				return errors.Wrapf(err, "couldn't close file '%s'", header.Name)
			}
		default:
			log.WithFields(log.Fields{"name": header.Name, "type": header.Typeflag}).Warn("Skipping unknown tar file type")
		}
	}
}

func (rm *RegistryManager) WaitForRunning(ctx context.Context, deployName string) error {
	var pods corev1.PodList
	opts := client.MatchingLabels{"name": deployName}

	err := retry.Do(
		func() error {
			numPodsRunning := 0
			err := rm.resourceMgr.List(ctx, defaultNamespace, &pods, opts)
			if err != nil {
				return errors.Wrap(err, "couldn't list pods of deployment")
			}
			if len(pods.Items) == 0 {
				return errors.New("pods are not yet up for the deployment")
			}

			expectedPodsRunning := len(pods.Items)
			for _, pod := range pods.Items {
				log.WithFields(log.Fields{"pod": pod.Name}).Info("Checking pod status")
				if pod.Status.Phase != corev1.PodRunning {
					log.WithFields(log.Fields{"pod": pod.Name}).Info("Waiting for pod to reach a running state")
				} else {
					log.WithFields(log.Fields{"pod": pod.Name}).Info("pod is in a running state")
					numPodsRunning++
				}

			}
			if numPodsRunning != expectedPodsRunning {
				return errors.New("timeout expired waiting for deployment")
			}
			return nil
		},
		retry.Delay(10*time.Second),
		retry.MaxDelay(10*time.Second),
		retry.LastErrorOnly(true),
		retry.Attempts(12),
	)
	if err != nil {
		return err
	}
	return nil
}
