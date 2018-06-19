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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/ligato/sfc-controller/plugins/controller/model"
)

const (
	CRDGroupName = "sfccontroller.ligato.github.com"

	CRDKindIpamPool = "IpamPool"
	CRDPluralIpamPool = "ipampools"

	CRDKindNetworkNode = "NetworkNode"
	CRDPluralNetworkNode = "networknodes"

	CRDKindNetworkNodeOverlay = "NetworkNodeOverlay"
	CRDPluralNetworkNodeOverlay = "networknodeoverlays"

	CRDKindNetworkService = "NetworkService"
	CRDPlurlaNetworkService = "networkservices"
)

// IpamPool is a specification for a IpamPool resource
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type IpamPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	controller.IPAMPoolSpec   `json:"spec,omitempty"`
	controller.IPAMPoolStatus `json:"status,omitempty"`
}

// IpamPoolList is a list of IpamPool resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type IpamPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []IpamPool `json:"items"`
}

// NetworkService is a specification for a NetworkService resource
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	controller.NetworkServiceSpec   `json:"spec,omitempty"`
	controller.NetworkServiceStatus `json:"status,omitempty"`
}

// NetworkServiceList is a list of NetworkService resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NetworkService `json:"items"`
}

// NetworkNodeOverlay is a specification for a NetworkNodeOverlay resource
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkNodeOverlay struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
    controller.NetworkNodeOverlaySpec   `json:"spec,omitempty"`
	controller.NetworkNodeOverlayStatus `json:"status,omitempty"`
}

// NetworkNodeOverlayList is a list of NetworkNodeOverlay resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkNodeOverlayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NetworkNodeOverlay `json:"items"`
}

// NetworkNode is a specification for a NetworkNode resource
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkNode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	controller.NetworkNodeSpec   `json:"spec,omitempty"`
	controller.NetworkNodeStatus `json:"status,omitempty"`
}

// NetworkNodeList is a list of NetworkNode resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NetworkNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []NetworkNode `json:"items"`
}
