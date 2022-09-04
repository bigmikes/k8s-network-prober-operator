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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkProberSpec defines the desired state of NetworkProber
type NetworkProberSpec struct {
	// PodSelector is the label selector to match Pods where the prober sidecar container is deployed.
	PodSelector metav1.LabelSelector `json:"podSelector"`
	// HttpTargets is the list of HTTP endpoints that the prober queries.
	HttpTargets []string `json:"httpTargets"`
}

// NetworkProberStatus defines the observed state of NetworkProber
type NetworkProberStatus struct {
	// Pods are the names of the pods where the prober sidecar container is deployed
	Pods []string `json:"pods"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// NetworkProber is the Schema for the networkprobers API
type NetworkProber struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkProberSpec   `json:"spec,omitempty"`
	Status NetworkProberStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NetworkProberList contains a list of NetworkProber
type NetworkProberList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkProber `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NetworkProber{}, &NetworkProberList{})
}
