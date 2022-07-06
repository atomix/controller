// SPDX-FileCopyrightText: 2022-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Protocol is a specification for a Protocol resource
type Protocol struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Versions []ProtocolVersion `json:"versions,omitempty"`
}

type ProtocolVersion struct {
	Name       string           `json:"name"`
	Primitives []PrimitiveKind  `json:"primitives"`
	Drivers    []ProtocolDriver `json:"drivers"`
}

type PrimitiveKind struct {
	Kind        string                `json:"kind"`
	APIVersions []PrimitiveAPIVersion `json:"apiVersions"`
}

type PrimitiveAPIVersion struct {
	Name    string `json:"name"`
	Service string `json:"service"`
}

type ProtocolDriver struct {
	RuntimeVersion string `json:"runtimeVersion"`
	Image          string `json:"image"`
	Path           string `json:"path"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProtocolList is a list of Protocol resources
type ProtocolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Protocol `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Store is a specification for a Store resource
type Store struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StoreSpec   `json:"spec"`
	Status StoreStatus `json:"status"`
}

// StoreSpec is the spec for a Store resource
type StoreSpec struct {
	Protocol ProtocolReference    `json:"protocol"`
	Config   runtime.RawExtension `json:"config"`
}

type ProtocolReference struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type StoreStatus struct {
	Primitives []PrimitiveStatus `json:"primitives"`
}

type PrimitiveStatus struct {
	corev1.ObjectReference `json:",inline"`
	CreationTimestamp      metav1.Time  `json:"creationTimestamp"`
	DeletionTimestamp      *metav1.Time `json:"deletionTimestamp"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// StoreList is a list of Store resources
type StoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Store `json:"items"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Profile is a specification for a Profile resource
type Profile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProfileSpec   `json:"spec"`
	Status ProfileStatus `json:"status"`
}

// ProfileSpec is the spec for a Profile resource
type ProfileSpec struct {
	Bindings []ProfileBinding `json:"bindings"`
}

type ProfileBinding struct {
	Name       string                 `json:"name"`
	Store      corev1.ObjectReference `json:"store"`
	Primitives []PrimitiveBindingRule `json:"primitives"`
}

type PrimitiveBindingRule struct {
	Kinds       []string          `json:"kinds"`
	APIVersions []string          `json:"apiVersions"`
	Names       []string          `json:"names"`
	Metadata    map[string]string `json:"metadata"`
}

type ProfileStatus struct {
	PodStatuses []ProfilePodStatus `json:"podStatuses"`
}

type ProfilePodStatus struct {
	corev1.ObjectReference `json:",inline"`
	Services               []ProfileServiceStatus `json:"services"`
	Sessions               []ProfileSessionStatus `json:"sessions"`
}

type ProfileServiceState string

const (
	ProfileServiceStarting = "Starting"
	ProfileServiceRunning  = "Running"
	ProfileServiceStopping = "Stopping"
	ProfileServiceStopped  = "Stopped"
)

type ProfileServiceStatus struct {
	Store corev1.ObjectReference `json:"store"`
	State ProfileServiceState    `json:"state"`
}

type ProfileSessionState string

const (
	ProfileSessionUnbound = "Unbound"
	ProfileSessionBound   = "Bound"
)

type ProfileSessionStatus struct {
	corev1.ObjectReference `json:",inline"`
	Service                string `json:"service"`
	Metadata               map[string][]string
	CreationTimestamp      metav1.Time         `json:"creationTimestamp"`
	DeletionTimestamp      *metav1.Time        `json:"deletionTimestamp"`
	State                  ProfileSessionState `json:"state"`
	Binding                string              `json:"binding"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProfileList is a list of Profile resources
type ProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Profile `json:"items"`
}
