package edgewize

import (
	"context"
	"fmt"
	synccontext "github.com/loft-sh/vcluster/pkg/controllers/syncer/context"
	"github.com/loft-sh/vcluster/pkg/util/translate"
	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	fakenodes = &sync.Map{}
	once      = sync.Once{}
)

func IsSystemWorkspace(cli client.Client, name string) (bool, error) {
	namespace := &corev1.Namespace{}
	err := cli.Get(context.Background(), types.NamespacedName{Name: name}, namespace)
	if err != nil {
		return false, err
	}
	return namespace.Labels["kubesphere.io/workspace"] == "system-workspace", nil
}

func IsPodNeedSync(cli client.Client, pod *corev1.Pod) bool {
	_, ok := pod.GetLabels()["edgewize.io/ignore-sync-pod"]
	if ok {
		fmt.Println(fmt.Sprintf("for pod %s/%s ignore sync is %v", pod.Namespace, pod.Name, ok))
		return false
	}
	ref := metav1.GetControllerOf(pod)
	pl := getParentLabel(cli, pod.Namespace, ref)
	_, ok = pl["edgewize.io/ignore-sync-pod"]
	fmt.Println(fmt.Sprintf("for %s %s/%s ignore sync is %v", ref.Kind, pod.Namespace, ref.Name, ok))
	return !ok
}

func getParentLabel(cli client.Client, namespace string, ref *metav1.OwnerReference) map[string]string {
	var label = make(map[string]string)
	var obj client.Object
	var objKey = types.NamespacedName{
		Namespace: namespace,
		Name:      ref.Name,
	}
	switch ref.Kind {
	case "CronJob":
		obj = new(batchv1.Job)
	case "DaemonSet":
		obj = new(appv1.DaemonSet)
	case "Deployment":
		obj = new(appv1.Deployment)
	case "Job":
		obj = new(batchv1.Job)
	case "ReplicaSet":
		var rls *appv1.ReplicaSet
		err := cli.Get(context.Background(), objKey, rls)
		if err == nil {
			return getParentLabel(cli, namespace, metav1.GetControllerOf(rls))
		}
	case "StatefulSet":
		obj = new(appv1.StatefulSet)
	default:
		return label
	}
	err := cli.Get(context.Background(), objKey, obj)
	if err != nil || obj == nil {
		return label
	}
	return obj.GetLabels()
}

func IsFakeNode(cli client.Client, name string) (bool, error) {
	if _, ok := fakenodes.Load(name); ok {
		return true, nil
	}
	node := &corev1.Node{}
	err := cli.Get(context.Background(), types.NamespacedName{Name: name}, node)
	if err != nil {
		klog.Errorf("failed to get node %s: %v", name, err)
		return false, err
	} else {
		if node.Labels["vcluster.loft.sh/fake-node"] == "true" {
			klog.Errorf("node is not fake node, but has label %s", name)
		}
	}
	return false, nil
}

func AddFakeNode(name string) {
	fakenodes.Store(name, struct{}{})
}

func InitFakeNode(ctx *synccontext.RegisterContext) {
	once.Do(func() {
		podList := &corev1.PodList{}
		err := ctx.PhysicalManager.GetClient().List(ctx.Context, podList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(map[string]string{
				"vcluster.loft.sh/managed-by": translate.Suffix,
			}),
		})
		if err != nil {
			klog.Errorf("error listing pods: %v", err)
		} else {
			klog.Infof("edgewize.FakeNodes found %d pods", len(podList.Items))
			for _, pod := range podList.Items {
				if pod.Spec.NodeName != "" {
					AddFakeNode(pod.Spec.NodeName)
					klog.Infof("edgewize.FakeNodes added fake node %s", pod.Spec.NodeName)
				}
			}
		}
	})
}
