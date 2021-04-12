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

// SubnetSpec defines the desired state of Subnet
type SubnetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//VpcRef to get the vpc Id, this will be ignored when vpcId is not empty
	VpcRef *VpcRef `json:"vpcRef,omitempty"`

	//VpcId to create the subnet
	VpcId *string `json:"vpcId,omitempty"`

	// Subnet Name name of the vpc
	SubnetName *string `json:"subnetName,omitempty"`

	// Subnet cidr，only have three valid range: 10.0.0.0/16，172.16.0.0/16，192.168.0.0/16
	CidrBlock *string `json:"cidrBlock,omitempty"`

	// Zone, true or false
	Zone *string `json:"zone,omitempty"`

	// DomainName。
	DomainName *string `json:"domainName,omitempty"`

	// Region to create the vpc
	Region *string `json:"region,omitempty"`

	// Tags：[{"Key": "city", "Value": "shanghai"}]。
	Tags []Tag `json:"tags,omitempty"`
}

type VpcRef struct {
	//The name to refer from, we don't allow to ref from another namespace
	Name *string `json:"name,omitempty"`
}

// SubnetStatus defines the observed state of Subnet
type SubnetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status     *string    `json:"status,omitempty"`
	Reason     *string    `json:"reason,omitempty"`
	SubnetId   *string    `json:"subnetId,omitempty"`
	Code       *string    `json:"code,omitempty"`
	LastRetry  *string    `json:"lastRetry,omitempty"`
	RetryCount *int       `json:"retryCount,omitempty"`
	Vpc        *VpcStatus `json:"vpc,omitempty"`
}

// +kubebuilder:object:root=true

// Subnet is the Schema for the subnets API
type Subnet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SubnetSpec   `json:"spec,omitempty"`
	Status SubnetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SubnetList contains a list of Subnet
type SubnetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Subnet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Subnet{}, &SubnetList{})
}
