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
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	probesv1alpha1 "github.com/bigmikes/k8s-network-prober-operator/api/v1alpha1"
)

// NetworkProberReconciler reconciles a NetworkProber object
type NetworkProberReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	netProberSet map[types.NamespacedName]probesv1alpha1.NetworkProber
}

func NewNetworkProberReconciler(clt client.Client, scheme *runtime.Scheme) *NetworkProberReconciler {
	return &NetworkProberReconciler{
		Client:       clt,
		Scheme:       scheme,
		netProberSet: make(map[types.NamespacedName]probesv1alpha1.NetworkProber),
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkProberReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&probesv1alpha1.NetworkProber{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}

//+kubebuilder:rbac:groups=probes.bigmikes.io,resources=networkprobers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=probes.bigmikes.io,resources=networkprobers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=probes.bigmikes.io,resources=networkprobers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

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

	netProber, deleted, err := r.getNetProber(ctx, req)
	if err != nil {
		log.Error(err, "failed to get network prober")
		return ctrl.Result{}, err
	}
	_ = deleted

	// TODO handle delete case
	// TODO handle modify case

	// Create ConfigMap
	configMap := &corev1.ConfigMap{}
	configMapExists := true
	err = r.Get(ctx, req.NamespacedName, configMap)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// ConfigMap does not exist, create it
			configMapExists = false
		}
	}

	// Find all matching Pod and register their IP address in the ConfigMap
	podList := &corev1.PodList{}
	selector := labels.SelectorFromSet(netProber.Spec.PodSelector.MatchLabels)
	listOption := client.MatchingLabelsSelector{
		Selector: selector,
	}
	err = r.List(ctx, podList, listOption)
	if err != nil {
		log.Error(err, "failed to list pods")
		return ctrl.Result{Requeue: true}, err
	}
	endpoints := []corev1.PodIP{}
	for _, pod := range podList.Items {
		log.Info("Matching pod", "name", pod.Name)
		endpoints = append(endpoints, pod.Status.PodIPs[0])
	}

	endpointsJSON, err := json.Marshal(endpoints)
	if err != nil {
		log.Error(err, "failed to marshal endpoints list")
		return ctrl.Result{}, err
	}

	configMap.Name = netProber.Name
	configMap.Namespace = netProber.Namespace
	configMap.BinaryData = map[string][]byte{
		"endpointsJSON": endpointsJSON,
	}
	if configMapExists {
		err = r.Update(ctx, configMap)
	} else {
		err = r.Create(ctx, configMap)
	}

	if err != nil {
		log.Error(err, "failed to create or update configmap")
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, err
		}
	}

	return ctrl.Result{}, nil
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
