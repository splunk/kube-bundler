// Copied from https://github.com/kubernetes/kubectl/blob/dc0b35e84e2088c9bde0e05e7589235fe69b5b67/pkg/polymorphichelpers/rollout_status.go

package rolloutstatus

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	deploymentutil "github.com/splunk/kube-bundler/helpers/deploymentutil"
)

// DeploymentStatusViewer implements the StatusViewer interface.
type DeploymentStatusViewer struct{}

// DaemonSetStatusViewer implements the StatusViewer interface.
type DaemonSetStatusViewer struct{}

// StatefulSetStatusViewer implements the StatusViewer interface.
type StatefulSetStatusViewer struct{}

// Status returns a message describing deployment status, and a bool value indicating if the status is considered done.
func (s *DeploymentStatusViewer) Status(obj runtime.Unstructured, revision int64) (string, bool, error) {
	deployment := &appsv1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), deployment)
	if err != nil {
		return "", false, fmt.Errorf("failed to convert %T to %T: %v", obj, deployment, err)
	}

	if revision > 0 {
		deploymentRev, err := deploymentutil.Revision(deployment)
		if err != nil {
			return "", false, fmt.Errorf("cannot get the revision of deployment %q: %v", deployment.Name, err)
		}
		if revision != deploymentRev {
			return "", false, fmt.Errorf("desired revision (%d) is different from the running revision (%d)", revision, deploymentRev)
		}
	}
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		cond := deploymentutil.GetDeploymentCondition(deployment.Status, appsv1.DeploymentProgressing)
		if cond != nil && cond.Reason == deploymentutil.TimedOutReason {
			return "", false, fmt.Errorf("deployment %q exceeded its progress deadline", deployment.Name)
		}
		if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d out of %d new replicas have been updated...\n", deployment.Name, deployment.Status.UpdatedReplicas, *deployment.Spec.Replicas), false, nil
		}
		if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d old replicas are pending termination...\n", deployment.Name, deployment.Status.Replicas-deployment.Status.UpdatedReplicas), false, nil
		}
		if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for deployment %q rollout to finish: %d of %d updated replicas are available...\n", deployment.Name, deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas), false, nil
		}
		return fmt.Sprintf("deployment %q successfully rolled out\n", deployment.Name), true, nil
	}
	return fmt.Sprintf("Waiting for deployment spec update to be observed...\n"), false, nil
}

// Status returns a message describing daemon set status, and a bool value indicating if the status is considered done.
func (s *DaemonSetStatusViewer) Status(obj runtime.Unstructured, revision int64) (string, bool, error) {
	//ignoring revision as DaemonSets does not have history yet

	daemon := &appsv1.DaemonSet{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), daemon)
	if err != nil {
		return "", false, fmt.Errorf("failed to convert %T to %T: %v", obj, daemon, err)
	}

	if daemon.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		return "", true, fmt.Errorf("rollout status is only available for %s strategy type", appsv1.RollingUpdateStatefulSetStrategyType)
	}
	if daemon.Generation <= daemon.Status.ObservedGeneration {
		if daemon.Status.UpdatedNumberScheduled < daemon.Status.DesiredNumberScheduled {
			return fmt.Sprintf("Waiting for daemon set %q rollout to finish: %d out of %d new pods have been updated...\n", daemon.Name, daemon.Status.UpdatedNumberScheduled, daemon.Status.DesiredNumberScheduled), false, nil
		}
		if daemon.Status.NumberAvailable < daemon.Status.DesiredNumberScheduled {
			return fmt.Sprintf("Waiting for daemon set %q rollout to finish: %d of %d updated pods are available...\n", daemon.Name, daemon.Status.NumberAvailable, daemon.Status.DesiredNumberScheduled), false, nil
		}
		return fmt.Sprintf("daemon set %q successfully rolled out\n", daemon.Name), true, nil
	}
	return fmt.Sprintf("Waiting for daemon set spec update to be observed...\n"), false, nil
}

// Status returns a message describing statefulset status, and a bool value indicating if the status is considered done.
func (s *StatefulSetStatusViewer) Status(obj runtime.Unstructured, revision int64) (string, bool, error) {
	sts := &appsv1.StatefulSet{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), sts)
	if err != nil {
		return "", false, fmt.Errorf("failed to convert %T to %T: %v", obj, sts, err)
	}

	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		return "", true, fmt.Errorf("rollout status is only available for %s strategy type", appsv1.RollingUpdateStatefulSetStrategyType)
	}
	if sts.Status.ObservedGeneration == 0 || sts.Generation > sts.Status.ObservedGeneration {
		return "Waiting for statefulset spec update to be observed...\n", false, nil
	}
	if sts.Spec.Replicas != nil && sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		return fmt.Sprintf("Waiting for %d pods to be ready...\n", *sts.Spec.Replicas-sts.Status.ReadyReplicas), false, nil
	}
	if sts.Spec.UpdateStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType && sts.Spec.UpdateStrategy.RollingUpdate != nil {
		if sts.Spec.Replicas != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
			if sts.Status.UpdatedReplicas < (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition) {
				return fmt.Sprintf("Waiting for partitioned roll out to finish: %d out of %d new pods have been updated...\n",
					sts.Status.UpdatedReplicas, *sts.Spec.Replicas-*sts.Spec.UpdateStrategy.RollingUpdate.Partition), false, nil
			}
		}
		return fmt.Sprintf("partitioned roll out complete: %d new pods have been updated...\n",
			sts.Status.UpdatedReplicas), true, nil
	}
	if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
		return fmt.Sprintf("waiting for statefulset rolling update to complete %d pods at revision %s...\n",
			sts.Status.UpdatedReplicas, sts.Status.UpdateRevision), false, nil
	}
	return fmt.Sprintf("statefulset rolling update complete %d pods at revision %s...\n", sts.Status.CurrentReplicas, sts.Status.CurrentRevision), true, nil

}