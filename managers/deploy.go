package managers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/splunk/kube-bundler/api/v1alpha1"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

const (
	ActionApplyOutputs = "apply outputs"
	ActionApply        = "apply"
	ActionDiff         = "diff"
	ActionDelete       = "delete"
	ActionWait         = "wait"
	ActionSmoketest    = "smoketest"
	ActionOutput       = "outputs"

	ParametersFile = "parameters.json"
	InstallFile    = "install.json"
	RequiresFile   = "requires.json"
	FlavorFile     = "flavor.json"
)

type DeployInfo struct {
	Name      string
	Namespace string
	Action    string
	Timeout   time.Duration

	configMap      string
	image          string
	dockerRegistry string
	parameters     []v1alpha1.ParameterSpec
	definitions    []v1alpha1.ParameterDefinitionSpec
	requires       []v1alpha1.RequiresList
	installSpec    v1alpha1.InstallSpec
	flavorSpec     v1alpha1.FlavorSpec
}

type DeployOpts struct {
	Action  string
	Timeout time.Duration
}

type DeployManager struct {
	kbClient             KBClient
	resourceMgr          *ResourceManager
	rolloutStatusManager *RolloutStatusManager
}

func NewDeployManager(c KBClient) *DeployManager {
	return &DeployManager{
		kbClient:             c,
		resourceMgr:          NewResourceManager(c),
		rolloutStatusManager: NewRolloutStatusManager(c),
	}
}

func (dm *DeployManager) Deploy(ctx context.Context, installRef InstallReference, deployOpts DeployOpts, showLogs bool) error {
	deployInfo := DeployInfo{
		Name:      installRef.Name,
		Namespace: installRef.Namespace,
		Action:    deployOpts.Action,
		Timeout:   deployOpts.Timeout,
	}

	var install v1alpha1.Install
	err := dm.resourceMgr.Get(ctx, deployInfo.Name, deployInfo.Namespace, &install)
	if err != nil {
		return errors.Wrapf(err, "couldn't get install %q", deployInfo.Name)
	}

	var flavor v1alpha1.Flavor
	err = dm.resourceMgr.Get(ctx, install.Spec.Flavor, "default", &flavor)
	if err != nil {
		return errors.Wrapf(err, "couldn't get flavor %q", install.Spec.Flavor)
	}

	appName := fmt.Sprintf("%s-%s", install.Spec.Application, install.Spec.Version)
	var app v1alpha1.Application
	err = dm.resourceMgr.Get(ctx, appName, deployInfo.Namespace, &app)
	if err != nil {
		return errors.Wrapf(err, "couldn't get Application %q", appName)
	}

	err = dm.validateRequiredParameters(installRef.Name, app.Spec.ParameterDefinitions, install.Spec.Parameters)
	if err != nil {
		return errors.Wrapf(err, "couldn't validate parameters for %q", deployInfo.Name)
	}

	// Use a custom cluster registry, if defined
	if install.Spec.DockerRegistry != "" {
		fullImage := "https://" + app.Spec.DeployImage
		u, err := url.Parse(fullImage)
		if err != nil {
			return errors.Wrapf(err, "couldn't parse docker image URL for deployImage '%s'", app.Spec.DeployImage)
		}
		deployInfo.image = path.Join(install.Spec.DockerRegistry, u.Path)
		deployInfo.dockerRegistry = install.Spec.DockerRegistry

		log.WithFields(log.Fields{"old": app.Spec.DeployImage, "new": deployInfo.image}).Debug("rewrote deployImage for cluster local registry")
	} else { // Use the default registry
		deployInfo.image = app.Spec.DeployImage
		deployInfo.dockerRegistry = app.Spec.DockerRegistry
		install.Spec.DockerRegistry = app.Spec.DockerRegistry
	}

	// Expose the calculated deployImage so it is available in install.json
	if install.Spec.DeployImage == "" {
		install.Spec.DeployImage = deployInfo.image
	}

	// Populate private deployInfo struct members
	deployInfo.configMap = getResourceName(deployInfo.Name, "") + "-config"
	deployInfo.parameters = install.Spec.Parameters
	deployInfo.definitions = app.Spec.ParameterDefinitions
	deployInfo.requires = app.Spec.Requires
	deployInfo.installSpec = install.Spec
	deployInfo.flavorSpec = flavor.Spec

	// Delete any existing job
	err = dm.DeleteJob(ctx, installRef, deployInfo.Action)
	if err != nil {
		return errors.Wrapf(err, "couldn't delete job for %q", deployInfo.Name)
	}

	// Update configmap
	err = dm.createOrPatchConfigmap(ctx, deployInfo)
	if err != nil {
		return errors.Wrapf(err, "couldn't create or update configmap for %q", deployInfo.Name)
	}

	// Create deploy job
	err = dm.createJob(ctx, deployInfo)
	if err != nil {
		return errors.Wrapf(err, "couldn't create job for %q", deployInfo.Name)
	}

	// Wait on deploy job
	err = dm.pollJob(ctx, deployInfo, installRef, showLogs)
	if err != nil {
		return errors.Wrapf(err, "couldn't poll job for %q", deployInfo.Name)
	}

	// Wait on resources
	if deployInfo.Action != ActionDelete {
		err = dm.rolloutStatusManager.Wait(ctx, installRef, deployInfo.Timeout)
		if err != nil {
			return errors.Wrapf(err, "failed waiting for resources for %q", deployInfo.Name)
		}
	}

	return nil
}

func (dm *DeployManager) validateRequiredParameters(installName string, definitions []v1alpha1.ParameterDefinitionSpec, parameters []v1alpha1.ParameterSpec) error {
	pm := NewParameterManager(dm.kbClient, installName, definitions, parameters)
	return pm.Validate()
}

// GetLogs returns the logs for the deploy container. The returned ReadCloser should be consumed and closed
func (dm *DeployManager) GetLogs(ctx context.Context, installRef InstallReference) (io.ReadCloser, error) {
	clientset := dm.kbClient.Interface

	// Get job
	job, err := clientset.BatchV1().Jobs(installRef.Namespace).Get(ctx, installRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't fetch job %q", installRef.Name)
	}

	// Get matching pods
	podInterface := clientset.CoreV1().Pods(installRef.Namespace)
	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(job.Spec.Selector.MatchLabels).String(),
	}

	// Wait until the pod is in a running state or the context expires
	expiry, deadlineExists := ctx.Deadline()
	var pod corev1.Pod
	for {
		podList, err := podInterface.List(ctx, listOptions)
		if err != nil {
			return nil, errors.Wrap(err, "error list pods for job")
		}

		if len(podList.Items) == 0 {
			log.WithField("installName", installRef.Name).Debug("no matching pods for install")
			if deadlineExists && time.Now().After(expiry) {
				return nil, errors.New("timeout expired while waiting for job pods to be created")
			}
			time.Sleep(time.Second)
			continue
		}

		pod = podList.Items[len(podList.Items)-1]
		log.WithFields(log.Fields{"pod": pod.Name, "phase": pod.Status.Phase}).Debug("Found pod for logs")

		if pod.Status.Phase != corev1.PodPending {
			break
		}

		if deadlineExists && time.Now().After(expiry) {
			return nil, errors.New("timeout expired while waiting for pod to become ready")
		}
		time.Sleep(time.Second)
	}

	// Use the first container of the first pod. This should give all the results from the deploy container, but could miss results in other situations.
	containerName := pod.Spec.Containers[0].Name

	podLogOpts := &corev1.PodLogOptions{
		Container: containerName, // empty string will raise error during flag validation
		Follow:    true,
	}
	req := podInterface.GetLogs(pod.Name, podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening stream for pod %s container %s", pod.Name, containerName)
	}

	return podLogs, nil
}

func (dm *DeployManager) DeleteJob(ctx context.Context, installRef InstallReference, action string) error {
	job := batchv1.Job{}
	jobName := installRef.Name
	if action == ActionSmoketest {
		jobName += "-smoketest"
	}

	err := dm.resourceMgr.Delete(ctx, jobName, installRef.Namespace, &job)
	if err != nil {
		return errors.Wrap(err, "couldn't delete job")
	}
	return nil
}

func (dm *DeployManager) Delete(ctx context.Context, installRef InstallReference) error {
	err := dm.DeleteJob(ctx, installRef, ActionApply)
	if err != nil {
		return err
	}

	err = dm.DeleteJob(ctx, installRef, ActionSmoketest)
	if err != nil {
		return err
	}

	var install v1alpha1.Install
	err = dm.resourceMgr.Delete(ctx, installRef.Name, installRef.Namespace, &install)
	if err != nil {
		return errors.Wrap(err, "couldn't delete install")
	}
	return nil
}

func (dm *DeployManager) createOrPatchConfigmap(ctx context.Context, deployInfo DeployInfo) error {
	pm := NewParameterManager(dm.kbClient, deployInfo.Name, deployInfo.definitions, deployInfo.parameters)
	m, err := pm.GetMergedMap()
	if err != nil {
		return errors.Wrap(err, "couldn't get merged map")
	}

	parametersJson, err := json.Marshal(m)
	if err != nil {
		return errors.Wrap(err, "couldn't encode parameters to json")
	}

	installJson, err := json.Marshal(deployInfo.installSpec)
	if err != nil {
		return errors.Wrap(err, "couldn't encode install spec to json")
	}

	requiresJson, err := json.Marshal(deployInfo.requires)
	if err != nil {
		return errors.Wrap(err, "couldn't encode require list to json")
	}

	flavorJson, err := json.Marshal(deployInfo.flavorSpec)
	if err != nil {
		return errors.Wrap(err, "couldn't encode flavor spec to json")
	}

	cm := &corev1.ConfigMap{}
	err = dm.resourceMgr.CreateOrPatch(ctx, deployInfo.configMap, deployInfo.Namespace, cm, func() error {
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data[ParametersFile] = string(parametersJson)
		cm.Data[InstallFile] = string(installJson)
		cm.Data[RequiresFile] = string(requiresJson)
		cm.Data[FlavorFile] = string(flavorJson)

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "couldn't create or update configmap")
	}

	return nil
}

func (dm *DeployManager) createJob(ctx context.Context, deployInfo DeployInfo) error {
	backoffLimit := int32(0)
	runAsNonRoot := false
	securityContext := corev1.SecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}
	nameWithAction := getNameWithAction(deployInfo.Name, deployInfo.Action)

	// Setup parameter volume mount
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      nameWithAction + "-config",
			MountPath: "/config",
		},
	}

	// Setup volume mounts for required inputs
	for _, require := range deployInfo.requires {
		baseName := getResourceName(require.Name, require.Suffix)

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      baseName + "-config",
			MountPath: "/config/inputs/" + baseName,
		})
	}

	containers := []corev1.Container{
		{
			Name:  nameWithAction,
			Image: deployInfo.image,
			//EnvFrom:         envFrom,
			//Command: []string{"bash"},
			Args: strings.Split(deployInfo.Action, " "),
			//Args:            []string{"-c", "sleep 3600"},
			SecurityContext: &securityContext,
			VolumeMounts:    volumeMounts,
		},
	}

	// Setup parameter volume
	volumes := []corev1.Volume{
		{
			Name: nameWithAction + "-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: deployInfo.configMap,
					},
				},
			},
		},
	}

	// Setup required inputs volumes
	for _, require := range deployInfo.requires {
		baseName := getResourceName(require.Name, require.Suffix)
		configmapName := baseName + "-config"

		volumes = append(volumes, corev1.Volume{
			Name: configmapName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configmapName,
					},
				},
			},
		},
		)
		log.WithFields(log.Fields{"name": require.Name, "suffix": require.Suffix, "configmap": configmapName}).Debug("Mounting input volume")
	}

	gracePeriod := int64(1)
	deadline := int64(deployInfo.Timeout/time.Second) + gracePeriod*2
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nameWithAction,
			Namespace: deployInfo.Namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &deadline,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ActiveDeadlineSeconds:         &deadline,
					TerminationGracePeriodSeconds: &gracePeriod,
					RestartPolicy:                 "Never", // ensures pod remains after job completes or fails
					Containers:                    containers,
					Volumes:                       volumes,
				},
			},
		},
	}

	err := dm.resourceMgr.Create(ctx, job)
	if err != nil {
		return errors.Wrap(err, "couldn't create job")
	}
	return nil
}

func (dm *DeployManager) pollJob(ctx context.Context, deployInfo DeployInfo, installRef InstallReference, showLogs bool) error {
	fmt.Printf("Waiting %v for action '%s' on %s...\n", deployInfo.Timeout, deployInfo.Action, deployInfo.Name)
	start := time.Now()

	var w io.Writer
	if showLogs {
		w = os.Stdout
	} else {
		w = io.Discard
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, deployInfo.Timeout)
	defer cancel()

	err := dm.printLogs(timeoutCtx, installRef, w)
	if err != nil {
		log.WithField("error", err).Error("Couldn't print logs")
	}

	// When the logs have finished streaming, the job should have completed or failed. Get the latest status
	var latestStatus batchv1.JobConditionType
	firstTry := true
	for firstTry || time.Since(start) < deployInfo.Timeout {
		job := &batchv1.Job{}
		err := dm.resourceMgr.Get(ctx, deployInfo.Name, deployInfo.Namespace, job)
		if err != nil {
			return errors.Wrap(err, "couldn't get job")
		}
		jobConditions := job.Status.Conditions
		if len(jobConditions) > 0 {
			latestStatus = jobConditions[len(jobConditions)-1].Type
			break
		}
		log.WithFields(log.Fields{"elapsed": time.Since(start).Round(time.Second)}).Debug("Waiting for final job status")
		time.Sleep(time.Second)
		firstTry = false
	}

	if latestStatus == batchv1.JobComplete {
		log.WithFields(log.Fields{"elapsed": time.Since(start).Round(time.Second)}).Info("Job complete")
		return nil
	}

	// Failure occurred, print logs if we're not already
	if !showLogs {
		err := dm.printLogs(ctx, installRef, os.Stdout)
		if err != nil {
			log.WithField("error", err).Error("Printing logs failed")
		}
	}
	log.WithFields(log.Fields{"elapsed": time.Since(start).Round(time.Second)}).Error("Job failed")

	if latestStatus == batchv1.JobFailed {
		return errors.New("deploy failed")
	}

	return errors.New("timeout expired")
}

// printLogs prints the latest logs from the given installRef.
func (dm *DeployManager) printLogs(ctx context.Context, installRef InstallReference, w io.Writer) error {
	out, err := dm.GetLogs(ctx, installRef)
	if err != nil {
		return errors.Wrap(err, "couldn't get logs")
	}
	defer out.Close()

	_, err = io.Copy(w, out)
	if err != nil {
		return errors.Wrap(err, "couldn't print logs from pod")
	}

	return nil
}
