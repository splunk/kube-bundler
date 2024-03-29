/*
   Copyright 2023 Splunk Inc.

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

// FlavorSpec defines the desired state of Flavor
type FlavorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name is the name of the HA flavor configuration
	Name string `json:"name"`

	// StatefulQuorumreplicas is the number of replicas on which a quorum-based stateful service should run
	StatefulQuorumReplicas int `json:"statefulQuorumReplicas"`

	// StatefulReplicationReplicas is the number of replicas on which a replication-based stateful service should run
	StatefulReplicationReplicas int `json:"statefulReplicationReplicas"`

	// StatelessReplicas is the number of replicas on which a stateless service should run
	StatelessReplicas int `json:"statelessReplicas"`

	// AntiAffinity determines whether services should apply required or optional anti-affinity
	// +kubebuilder:validation:Enum=required;optional
	AntiAffinity string `json:"antiAffinity"`

	// MinimumNodes determines how many nodes the flavor requires to install. This prevents installing on infrastructure that won’t support the HA requirements
	MinimumNodes int `json:"minimumNodes"`
}

// FlavorStatus defines the observed state of Flavor
type FlavorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Flavor is the Schema for the flavors API
type Flavor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FlavorSpec   `json:"spec,omitempty"`
	Status FlavorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FlavorList contains a list of Flavor
type FlavorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Flavor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Flavor{}, &FlavorList{})
}
