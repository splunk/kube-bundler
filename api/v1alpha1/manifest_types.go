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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ManifestSpec defines the desired state of Manifest
type ManifestSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Flavor is the cluster configuration
	Flavor string `json:"flavor,omitempty"`

	// Registry is the cluster local registry to import images
	Registry string `json:"registry,omitempty"`

	// Sources is the list of sources to use for bundle files
	Sources []SourceInfo `json:"sources"`

	// Bundles is the list of bundles to install
	Bundles []BundleSpec `json:"bundles"`

	// CPU is the required cluster CPU
	CPU string `json:"cpu,omitempty"`

	// Memory is the required cluster memory
	Memory string `json:"memory,omitempty"`
}

type BundleSpec struct {
	Name       string          `json:"name"`
	Version    string          `json:"version"`
	Parameters []ParameterSpec `json:"parameters,omitempty"`
	Requires   []RequiresList  `json:"requires,omitempty"`
}

type SourceInfo struct {
	Name    string `json:"name"`
	Section string `json:"section,omitempty"`
	Release string `json:"release,omitempty"`
}

// ManifestStatus defines the observed state of Manifest
type ManifestStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Manifest is the Schema for the manifests API
type Manifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManifestSpec   `json:"spec,omitempty"`
	Status ManifestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ManifestList contains a list of Manifest
type ManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Manifest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Manifest{}, &ManifestList{})
}
