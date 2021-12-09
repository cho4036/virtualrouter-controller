/*
Copyright 2017 The Kubernetes Authors.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualRouter is a specification for a VirtualRouter resource
type VirtualRouter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualRouterSpec   `json:"spec"`
	Status VirtualRouterStatus `json:"status"`
}

// VirtualRouterSpec is the spec for a VirtualRouter resource
type VirtualRouterSpec struct {
	DeploymentName string         `json:"deploymentName"`
	Replicas       *int32         `json:"replicas"`
	VlanNumber     int32          `json:"vlanNumber" `
	InternalIPs    []string       `json:"internalIPs"`
	ExternalIPs    []string       `json:"externalIPs"`
	InternalCIDR   string         `json:"internalCIDR"`
	Image          string         `json:"image"`
	NodeSelector   []NodeSelector `json:"nodeSelector"`
}

// VirtualRouterStatus is the status for a VirtualRouter resource
type VirtualRouterStatus struct {
	AvailableReplicas int32           `json:"availableReplicas"`
	ReplicaStatus     []ReplicaStatus `json:"replicaStatus"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// VirtualRouterList is a list of VirtualRouter resources
type VirtualRouterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []VirtualRouter `json:"items"`
}

type NodeSelector struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ReplicaStatus struct {
	Scheduled bool   `json:"scheduled"`
	PodName   string `json:"podName"`
	NodeName  string `json:"hostName"`
	Bridged   bool   `json:"bridged"`
	// Scheduling -> Scheduled -> Bridging -> Bridged -> Running -> UnScheduling -> UnScheduled -> UnBridging -> UnBridged -> Removed
	Phase string `json:"phase"`
}

type Phase string

var (
	SCHEDULING   Phase = "Scheduling"
	SCHEDULED    Phase = "Scheduled"
	BRIDGING     Phase = "Bridging"
	BRIDGED      Phase = "Bridged"
	RUNNING      Phase = "Running"
	UNSCHEDULING Phase = "UnScheduling"
	UNSCHEDULED  Phase = "UnScheduled"
	UNBRIDGING   Phase = "UnBridging"
	UNBRIDGED    Phase = "UnBridged"
	REMOVED      Phase = "Removed"
	REMOVING     Phase = "Removing"
)
