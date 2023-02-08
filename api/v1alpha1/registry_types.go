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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// RegistrySpec defines the desired state of Registry
type RegistrySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Image is the registry image reference
	Image string `json:"image"`

	// Flavor is the name of the flavor that will determine the number of replicas to deploy. If left empty,
	// the flavor called "default" will be used.
	Flavor string `json:"flavor,omitempty"`

	// NodeSelector contains the node labels to apply to this deployment
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// HostPath is the path on the host used to store the registry contents. Defaults to /var/lib/registry/<registry-name>
	HostPath string `json:"hostPath,omitempty"`
}

// RegistryStatus defines the observed state of Registry
type RegistryStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Registry is the Schema for the registries API
type Registry struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistrySpec   `json:"spec,omitempty"`
	Status RegistryStatus `json:"status,omitempty"`
}

func (r *Registry) ClusterUrl() string {
	regName := r.SanitizedRegistryName()
	registryUrl := fmt.Sprintf("localhost:6000/registry-%s", regName)
	return registryUrl
}

func (r *Registry) SanitizedRegistryName() string {
	regName := r.ObjectMeta.Name
	invalidDnsChars := []string{".", "_", "^", "$", "@", "!", "+", "=", "(", ")", "&"}

	// Replace all invalid DNS characters in the registry name with the "-" char
	for _, invalidChar := range invalidDnsChars {
		regName = strings.ReplaceAll(regName, invalidChar, "-")
	}
	return regName
}

//+kubebuilder:object:root=true

// RegistryList contains a list of Registry
type RegistryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Registry `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Registry{}, &RegistryList{})
}
