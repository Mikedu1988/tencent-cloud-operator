/*


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

// VpcSpec defines the desired state of Vpc
type VpcSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// VpcName name of the vpc
	VpcName *string `json:"vpcName,omitempty"`

	// vpc cidr，only have three valid range: 10.0.0.0/16，172.16.0.0/16，192.168.0.0/16
	CidrBlock *string `json:"cidrBlock,omitempty"`

	// Multicast, true or false
	EnableMulticast *string `json:"enableMulticast,omitempty"`

	// DnsServers address, support up to 4 dns servers。
	DnsServers []*string `json:"dnsServers,omitempty"`

	// DomainName。
	DomainName *string `json:"domainName,omitempty"`

	// Region to create the vpc
	Region *string `json:"region,omitempty"`

	// Tags：[{"Key": "city", "Value": "shanghai"}]。
	Tags []Tag `json:"tags,omitempty"`
}

type Tag struct {
	Key   *string `json:"key,omitempty"`
	Value *string `json:"value,omitempty"`
}

// VpcStatus defines the observed state of Vpc
type VpcStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status *string `json:"status,omitempty"`
	VpcId  *string `json:"vpc_id,omitempty"`
}

// +kubebuilder:object:root=true

// Vpc is the Schema for the vpcs API
type Vpc struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VpcSpec   `json:"spec,omitempty"`
	Status VpcStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// VpcList contains a list of Vpc
type VpcList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Vpc `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Vpc{}, &VpcList{})
}
