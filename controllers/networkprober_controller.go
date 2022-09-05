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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	probesv1alpha1 "github.com/bigmikes/k8s-network-prober-operator/api/v1alpha1"
)

// NetworkProberReconciler reconciles a NetworkProber object
type NetworkProberReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	podSet       map[types.NamespacedName]corev1.Pod
	netProberSet map[types.NamespacedName]probesv1alpha1.NetworkProber
}

func NewNetworkProberReconciler(clt client.Client, scheme *runtime.Scheme) *NetworkProberReconciler {
	return &NetworkProberReconciler{
		Client:       clt,
		Scheme:       scheme,
		podSet:       make(map[types.NamespacedName]corev1.Pod),
		netProberSet: make(map[types.NamespacedName]probesv1alpha1.NetworkProber),
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkProberReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&probesv1alpha1.NetworkProber{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=probes.bigmikes.io,resources=networkprobers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=probes.bigmikes.io,resources=networkprobers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=probes.bigmikes.io,resources=networkprobers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NetworkProber object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *NetworkProberReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling", "name", req.NamespacedName)

	pod, deleted, err := r.getPod(ctx, req)
	if err != nil {
		log.Error(err, "failed to get pod")
		return ctrl.Result{}, err
	}
	if pod != nil {
		// Request contains a Pod
		log.Info("Got Pod", "deleted", deleted)
		return r.reconcilePod(ctx, pod, deleted)
	} else {
		netProber, deleted, err := r.getNetProber(ctx, req)
		if err != nil {
			log.Error(err, "failed to get net prober")
			return ctrl.Result{}, err
		}
		if netProber != nil {
			// Request contains a NetworkProber
			log.Info("Got NetworkProber", "deleted", deleted)
			return r.reconcileNetworkProber(ctx, netProber, deleted)
		}
	}

	return ctrl.Result{}, nil
}

func (r *NetworkProberReconciler) getPod(ctx context.Context, req ctrl.Request) (*corev1.Pod, bool, error) {
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

func (r *NetworkProberReconciler) getNetProber(ctx context.Context, req ctrl.Request) (*probesv1alpha1.NetworkProber, bool, error) {
	var netProber probesv1alpha1.NetworkProber
	err := r.Get(ctx, req.NamespacedName, &netProber)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if netProber, ok := r.netProberSet[req.NamespacedName]; ok {
				// This was a pod and now it has been deleted
				delete(r.netProberSet, req.NamespacedName)
				return &netProber, true, nil
			}
			return nil, false, nil
		}
		return nil, false, err
	}
	r.netProberSet[req.NamespacedName] = netProber
	return &netProber, false, nil
}

func (r *NetworkProberReconciler) reconcilePod(ctx context.Context, pod *corev1.Pod, deleted bool) (ctrl.Result, error) {

	return ctrl.Result{}, nil
}

func (r *NetworkProberReconciler) reconcileNetworkProber(ctx context.Context,
	netProber *probesv1alpha1.NetworkProber,
	deleted bool) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	// TODO handle delete case
	// TODO handle modify case

	podList := &corev1.PodList{}
	selector := labels.SelectorFromSet(netProber.Spec.PodSelector.MatchLabels)
	listOption := client.MatchingLabelsSelector{
		Selector: selector,
	}
	err := r.List(ctx, podList, listOption)
	if err != nil {
		log.Error(err, "failed to list pods")
		return ctrl.Result{}, err
	}
	for _, pod := range podList.Items {
		log.Info("Matching pod", "name", pod.Name)
	}

	return ctrl.Result{}, nil
}
