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
	v20170312 "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

//// SubnetSpec defines the desired state of Subnet
type SubnetSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//VpcRef to get the vpc Id
	VpcRef *VpcRef           `json:"vpcRef,omitempty"`
	Region *string           `json:"region,omitempty"`
	Subnet *v20170312.Subnet `json:"subnet,omitempty"`
}

type VpcRef struct {
	//The name to refer from, we don't allow to ref from another namespace
	Name *string `json:"name,omitempty"`
}

// SubnetStatus defines the observed state of Subnet
type SubnetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Status     *string        `json:"status,omitempty"`
	Reason     *string        `json:"reason,omitempty"`
	SubnetId   *string        `json:"subnetId,omitempty"`
	Code       *string        `json:"code,omitempty"`
	LastRetry  *string        `json:"lastRetry,omitempty"`
	RetryCount *int           `json:"retryCount,omitempty"`
	Vpc        *v20170312.Vpc `json:"vpc,omitempty"`
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

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubnetSpec) DeepCopyInto(out *SubnetSpec) {
	*out = *in
	if in.VpcRef != nil {
		in, out := &in.VpcRef, &out.VpcRef
		*out = new(VpcRef)
		(*in).DeepCopyInto(*out)
	}
	if in.Region != nil {
		in, out := &in.Region, &out.Region
		*out = new(string)
		**out = **in
	}
	if in.Subnet != nil {
		in, out := &in.Subnet, &out.Subnet
		*out = new(v20170312.Subnet)
		**out = **in
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SubnetStatus) DeepCopyInto(out *SubnetStatus) {
	*out = *in
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(string)
		**out = **in
	}
	if in.Reason != nil {
		in, out := &in.Reason, &out.Reason
		*out = new(string)
		**out = **in
	}
	if in.SubnetId != nil {
		in, out := &in.SubnetId, &out.SubnetId
		*out = new(string)
		**out = **in
	}
	if in.Code != nil {
		in, out := &in.Code, &out.Code
		*out = new(string)
		**out = **in
	}
	if in.LastRetry != nil {
		in, out := &in.LastRetry, &out.LastRetry
		*out = new(string)
		**out = **in
	}
	if in.RetryCount != nil {
		in, out := &in.RetryCount, &out.RetryCount
		*out = new(int)
		**out = **in
	}
	if in.Vpc != nil {
		in, out := &in.Vpc, &out.Vpc
		*out = new(v20170312.Vpc)
		**out = **in
	}
}
