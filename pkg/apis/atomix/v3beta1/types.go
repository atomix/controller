// SPDX-FileCopyrightText: 2022-present Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0

package v3beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cluster is a specification for a Cluster resource
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status"`
}

// ClusterSpec is the spec for a Cluster resource
type ClusterSpec struct {
	Driver Driver               `json:"driver,omitempty"`
	Config runtime.RawExtension `json:"config,omitempty"`
}

// ClusterStatus is the status for a Cluster resource
type ClusterStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList is a list of Cluster resources
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Cluster `json:"items"`
}

type Driver struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Binding is a specification for a Binding resource
type Binding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BindingSpec `json:"spec"`
}

// BindingSpec is the spec for a Binding resource
type BindingSpec struct {
	Cluster corev1.ObjectReference `json:"cluster,omitempty"`
	Rules   []BindingRule          `json:"rules,omitempty"`
}

type BindingRule struct {
	Kinds    []string          `json:"kinds,omitempty"`
	Names    []string          `json:"names,omitempty"`
	Metadata map[string]string `json:"metadata"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BindingList is a list of Binding resources
type BindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Binding `json:"items"`
}
