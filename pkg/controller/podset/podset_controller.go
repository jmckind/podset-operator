// Copyright 2018 PodSet Operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package podset

import (
	"context"
	"fmt"
	"reflect"

	operatorv1alpha1 "github.com/jmckind/podset-operator/pkg/apis/operator/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_podset")

// Add creates a new PodSet Controller and adds it to the Manager. The Manager
// will set fields on the Controller and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcilePodSet{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("podset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PodSet
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.PodSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner PodSet
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.PodSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcilePodSet{}

// ReconcilePodSet reconciles a PodSet object
type ReconcilePodSet struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a PodSet object and makes changes based on the state read
// and what is in the PodSet.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePodSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("PodSet.Namespace", request.Namespace, "PodSet.Name", request.Name)
	reqLogger.Info("Reconciling Starting")

	// Fetch the PodSet instance
	instance := &operatorv1alpha1.PodSet{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// List the Pods managed by the PodSet.
	pods, err := listPods(r, instance)
	if err != nil {
		return reconcile.Result{}, err
	}

	actualSize := int32(len(pods))
	expectedSize := instance.Spec.Size
	reqLogger.Info(fmt.Sprintf("%d Pods exist for PodSet %s", actualSize, instance.Name))

	// Add or remove a Pod from the PodSet, if needed.
	if expectedSize > 0 && actualSize < expectedSize {
		reqLogger.Info("Adding Pod")
		err = addPod(r, instance)
	} else if actualSize > 0 && actualSize > expectedSize {
		reqLogger.Info("Removing Pod")
		err = removePod(r, pods)
	}

	if err != nil {
		return reconcile.Result{}, err
	}

	// Update the status for the PodSet, if needed.
	err = updateStatus(r, instance, pods)
	if err != nil {
		return reconcile.Result{}, err
	}

	reqLogger.Info("Reconciling Complete")
	return reconcile.Result{}, nil
}

// addPod will create a new Pod based on the given PodSet.
func addPod(r *ReconcilePodSet, cr *operatorv1alpha1.PodSet) error {
	pod := newNginxPod(cr)
	err := controllerutil.SetControllerReference(cr, pod, r.scheme)
	if err != nil {
		return err
	}
	return r.client.Create(context.TODO(), pod)
}

// defaultLabels returns the default set of labels for the PodSet.
func defaultLabels(cr *operatorv1alpha1.PodSet) map[string]string {
	return map[string]string{
		"app":    "podset",
		"podset": cr.Name,
	}
}

// labelsForPodSet returns the combined, set of labels for the PodSet.
func labelsForPodSet(cr *operatorv1alpha1.PodSet) map[string]string {
	labels := defaultLabels(cr)
	for key, val := range cr.ObjectMeta.Labels {
		labels[key] = val
	}
	return labels
}

// listPods will return a slice containing the Pods owned by the Operator that
// do not have a DeletionTimestamp set.
func listPods(r *ReconcilePodSet, cr *operatorv1alpha1.PodSet) ([]corev1.Pod, error) {
	// List the pods for the given PodSet.
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForPodSet(cr))
	listOps := &client.ListOptions{Namespace: cr.Namespace, LabelSelector: labelSelector}
	err := r.client.List(context.TODO(), listOps, podList)
	if err != nil {
		return nil, err
	}

	// Filter out Pods with a DeletionTimestamp.
	pods := make([]corev1.Pod, 0)
	for _, pod := range podList.Items {
		if pod.DeletionTimestamp == nil {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

// newBusyboxPod returns a simple busybox pod for the given PodSet.
func newBusyboxPod(cr *operatorv1alpha1.PodSet) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", cr.Name),
			Namespace:    cr.Namespace,
			Labels:       labelsForPodSet(cr),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}

// newNginxPod returns a simple nginx pod for the given PodSet.
func newNginxPod(cr *operatorv1alpha1.PodSet) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", cr.Name),
			Namespace:    cr.Namespace,
			Labels:       labelsForPodSet(cr),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:stable-alpine",
				},
			},
		},
	}
}

// removePod will delete the first pod in the given slice of Pods.
func removePod(r *ReconcilePodSet, pods []corev1.Pod) error {
	return r.client.Delete(context.TODO(), &pods[0])
}

// updateStatus will update PodNames with the current list of Pods managed by the the PodSet.
func updateStatus(r *ReconcilePodSet, cr *operatorv1alpha1.PodSet, pods []corev1.Pod) error {
	podNames := make([]string, 0)
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}

	if reflect.DeepEqual(cr.Status.PodNames, podNames) {
		return nil
	}

	cr.Status.PodNames = podNames
	return r.client.Update(context.TODO(), cr)
}
