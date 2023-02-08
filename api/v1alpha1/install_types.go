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

// InstallSpec defines the desired state of Install
type InstallSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Application is the name of the application being deployed
	Application string `json:"application,omitempty"`

	// Version is the installation version
	Version string `json:"version"`

	// Suffix is the resource suffix applied to the install
	Suffix string `json:"suffix"`

	// DeployImage overrides the DeployImage from the application
	DeployImage string `json:"deployImage,omitempty"`

	// Flavor is the deployment flavor associated with an install
	Flavor string `json:"flavor,omitempty"`

	// DockerRegistry is the location of the desired registry. If non-empty, the application's image references will be rewritten
	// to use this registry.
	DockerRegistry string `json:"dockerRegistry,omitempty"`

	// Parameters are a list of installation configuration options
	Parameters []ParameterSpec `json:"parameters,omitempty"`

	// Secrets are a list of installation secrets
	Secrets []ParameterSpec `json:"secrets,omitempty"`
}

type ParameterSpec struct {
	Name           string         `json:"name"`
	Value          string         `json:"value"`
	GenerateSecret GenerateSecret `json:"generateSecret,omitempty"`
}

type GenerateSecret struct {
	Format string `json:"format" yaml:"format"`
	Bytes  int    `json:"bytes,omitempty"`
	Bits   int    `json:"bits,omitempty"`
}

// InstallStatus defines the observed state of Install
type InstallStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Install is the Schema for the installs API
type Install struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstallSpec   `json:"spec,omitempty"`
	Status InstallStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// InstallList contains a list of Install
type InstallList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Install `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Install{}, &InstallList{})
}
