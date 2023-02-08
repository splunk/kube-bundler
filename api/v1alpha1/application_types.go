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
	"errors"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ApplicationSpec defines the desired state of Application
type ApplicationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Name of the application
	Name string `json:"name,omitempty"`

	// Version of the application
	Version string `json:"version,omitempty"`

	// DockerRegistry determines where docker images should be pulled from when there is no cluster local registry. If airgap images
	// should always be used, this may be left blank.
	DockerRegistry string `json:"dockerRegistry,omitempty"`

	// DeployImage is the image used to perform deployment operations
	DeployImage string `json:"deployImage,omitempty"`

	// Images that should be bundled with the application
	Images []ImageSpec `json:"images,omitempty"`

	// ParameterDefinitions used by the application during deploy
	ParameterDefinitions []ParameterDefinitionSpec `json:"parameters"`

	// Provides lists the dependency provided by this application
	Provides []ProvidesList `json:"provides,omitempty"`

	// Requires lists the dependency required by this application
	Requires []RequiresList `json:"requires,omitempty"`

	// Resources defines the kubernetes resources associated with this application
	Resources []Resource `json:"resources,omitempty"`

	// Status defines application status
	Status []StatusList `json:"status,omitempty"`
}

type StatusList struct {
	// Endpoint is fully qualified URL
	Endpoint     string `json:"endpoint"`
	ExpectedCode string `json:"expectedCode,omitempty"`
}

type ImageSpec struct {
	// Image is the fully qualified path and tag
	Image string `json:"image"`

	// Scheme is either https or http. Defaults to https
	Scheme string `json:"scheme"`
}

type ParameterDefinitionSpec struct {
	Name           string         `json:"name,omitempty"`
	Default        string         `json:"default,omitempty"`
	Description    string         `json:"description,omitempty"`
	Required       bool           `json:"required,omitempty"`
	GenerateSecret GenerateSecret `json:"generateSecret,omitempty"`
}

type OutputDefinitionSpec struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type ProvidesList struct {
	Name    string                 `json:"name,omitempty"`
	Outputs []OutputDefinitionSpec `json:"outputs,omitempty"`
}

type RequiresList struct {
	Name       string          `json:"name"`
	Suffix     string          `json:"suffix"`
	Parameters []ParameterSpec `json:"parameters,omitempty"`
}

type Resource struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Type     string `json:"type"`
}

type ResourceList []Resource

// ApplicationStatus defines the observed state of Application
type ApplicationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Application is the Schema for the applications API
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApplicationSpec   `json:"spec,omitempty"`
	Status ApplicationStatus `json:"status,omitempty"`
}

func (a *Application) Validate() error {
	if strings.TrimSpace(a.Spec.Name) == "" {
		return errors.New("empty field 'name'")
	}
	if strings.TrimSpace(a.Spec.Version) == "" {
		return errors.New("empty field 'version'")
	}
	if strings.TrimSpace(a.Spec.DeployImage) == "" {
		return errors.New("empty field 'deployImage'")
	}
	return nil
}

//+kubebuilder:object:root=true

// ApplicationList contains a list of Application
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Application{}, &ApplicationList{})
}
