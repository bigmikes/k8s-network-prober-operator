/*
Copyright 2022.

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

package controllers

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	probesv1alpha1 "github.com/bigmikes/k8s-network-prober-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	podSet map[types.NamespacedName]corev1.Pod
}

func NewPodReconciler(clt client.Client, scheme *runtime.Scheme) *PodReconciler {
	return &PodReconciler{
		Client: clt,
		Scheme: scheme,
		podSet: make(map[types.NamespacedName]corev1.Pod),
	}
}

//+kubebuilder:rbac:groups=probes.bigmikes.io,resources=networkprobers,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=probes.bigmikes.io,resources=networkprobers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=probes.bigmikes.io,resources=networkprobers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Pod object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling", "name", req.NamespacedName)

	pod, deleted, err := r.getPod(ctx, req)
	if err != nil {
		log.Error(err, "failed to get pod")
		return ctrl.Result{}, err
	}
	_ = deleted

	// TODO handle delete case
	// TODO handle modify case

	netProberList := &probesv1alpha1.NetworkProberList{}
	err = r.List(ctx, netProberList)
	if err != nil {
		log.Error(err, "failed to list pods")
		return ctrl.Result{}, err
	}
	for _, netProber := range netProberList.Items {
		log.Info("Listed NetProber", "name", netProber.Name)
		selector := labels.SelectorFromSet(netProber.Spec.PodSelector.MatchLabels)
		if selector.Matches(labels.Set(pod.Labels)) {
			log.Info("Match", "pod", pod.Name, "netProber", netProber.Name)
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}

func (r *PodReconciler) getPod(ctx context.Context, req ctrl.Request) (*corev1.Pod, bool, error) {
	var pod corev1.Pod
	err := r.Get(ctx, req.NamespacedName, &pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if pod, ok := r.podSet[req.NamespacedName]; ok {
				// This was a pod and not it has been deleted
				delete(r.podSet, req.NamespacedName)
				return &pod, true, nil
			}
			return nil, false, nil
		}
		return nil, false, err
	}
	r.podSet[req.NamespacedName] = pod
	return &pod, false, nil
}
