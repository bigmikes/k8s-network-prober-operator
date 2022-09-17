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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	probesv1alpha1 "github.com/bigmikes/k8s-network-prober-operator/api/v1alpha1"
	"github.com/go-logr/logr"
)

type RequestType int

const (
	CRDCreated RequestType = iota
	CRDDeleted
	CRDUpdated
)

type Config struct {
	EndpointsMap  map[string]Endpoint `json:"endpointsMap"`
	PollingPeriod time.Duration       `json:"pollingPeriod"`
}

type Endpoint struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

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
// The NetworkProberReconcile discover Pods matching its podSelector property
// and populate the ConfigMap needed by the network-prober container to probe the Pods.
func (r *NetworkProberReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling", "name", req.NamespacedName)

	// Get the NetworkProber and the status of the object
	netProber, reqType, err := r.getNetProber(ctx, req)
	if err != nil {
		log.Error(err, "failed to get network prober")
		return ctrl.Result{}, err
	}

	switch reqType {
	case CRDCreated, CRDUpdated:
		log.Info("Handle create or ", "name", req.NamespacedName)
		return r.handleCreateUpdate(ctx, log, req.NamespacedName, netProber)
	default:
		log.Info("Handle delete", "name", req.NamespacedName)
		return r.handleDelete(ctx, log, req.NamespacedName, netProber)
	}
}

func (r *NetworkProberReconciler) getNetProber(ctx context.Context, req ctrl.Request) (*probesv1alpha1.NetworkProber, RequestType, error) {
	var netProber probesv1alpha1.NetworkProber
	err := r.Get(ctx, req.NamespacedName, &netProber)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if netProber, ok := r.netProberSet[req.NamespacedName]; ok {
				// Resource exists and now it has been deleted
				return &netProber, CRDDeleted, nil
			}
			return nil, CRDDeleted, nil
		}
		return nil, CRDDeleted, err
	}
	if _, ok := r.netProberSet[req.NamespacedName]; ok {
		// Resource already exists, this is an update case
		return &netProber, CRDUpdated, nil
	}
	return &netProber, CRDCreated, nil
}

func (r *NetworkProberReconciler) handleCreateUpdate(ctx context.Context,
	log logr.Logger,
	resName types.NamespacedName,
	netProber *probesv1alpha1.NetworkProber) (ctrl.Result, error) {
	// Fetch ConfigMap if exists
	configMap := &corev1.ConfigMap{}
	configMapExists := true
	err := r.Get(ctx, resName, configMap)
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
	config := Config{
		EndpointsMap: make(map[string]Endpoint),
	}
	for _, pod := range podList.Items {
		log.Info("Matching pod", "name", pod.Name)
		if len(pod.Status.PodIPs) == 0 {
			log.Info("Pod does not have IPs yet, requeueing")
			return ctrl.Result{Requeue: true}, nil
		}
		config.EndpointsMap[pod.Name] = Endpoint{
			IP:   pod.Status.PodIPs[0].IP,
			Port: netProber.Spec.HttpPort,
		}
	}
	// Populate the config with other needed properties
	config.PollingPeriod, err = time.ParseDuration(netProber.Spec.PollingPeriod)
	if err != nil {
		log.Error(err, "failed to parse duration of pollingPeriod")
		return ctrl.Result{}, err
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		log.Error(err, "failed to marshal endpoints list")
		return ctrl.Result{}, err
	}

	configMap.Name = netProber.Name
	configMap.Namespace = netProber.Namespace
	configMap.BinaryData = map[string][]byte{
		"config.json": configJSON,
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
	} else {
		// Create or update completed successfully, save the CRD in map
		r.netProberSet[resName] = *netProber
	}

	return ctrl.Result{}, err
}

func (r *NetworkProberReconciler) handleDelete(ctx context.Context,
	log logr.Logger,
	resName types.NamespacedName,
	netProber *probesv1alpha1.NetworkProber) (ctrl.Result, error) {

	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, resName, configMap)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// ConfigMap does not exist, no need to delete it
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, err
	}
	err = r.Delete(ctx, configMap)
	if err != nil {
		log.Error(err, "failed to delete configmap")
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, err
		}
	} else {
		// Deletion was successful, delete CRD from map
		delete(r.netProberSet, resName)
	}
	return ctrl.Result{}, err
}
