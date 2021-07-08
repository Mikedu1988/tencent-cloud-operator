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
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"tencent-cloud-operator/internal/common"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SecurityGroupSpec defines the desired state of SecurityGroup
type SecurityGroupSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of SecurityGroup. Edit SecurityGroup_types.go to remove/update
	Region                 *string                       `json:"region,omitempty"`
	SecurityGroup          *tcvpc.SecurityGroup          `json:"securityGroup,omitempty"`
	SecurityGroupPolicySet *tcvpc.SecurityGroupPolicySet `json:"securityGroupPolicySet,omitempty"`
}

// SecurityGroupStatus defines the observed state of SecurityGroup
type SecurityGroupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ResourceStatus               *common.ResourceStatus        `json:"resourceStatus,omitempty"`
	SecurityGroupStatus          *tcvpc.SecurityGroup          `json:"securityGroup,omitempty"`
	SecurityGroupPolicySetStatus *tcvpc.SecurityGroupPolicySet `json:"securityGroupPolicySet,omitempty"`
}

// SecurityGroup is the Schema for the securitygroups API
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="SecurityGroupId",type=string,JSONPath=`.status.securityGroup.SecurityGroupId`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.resourceStatus.status`
type SecurityGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecurityGroupSpec   `json:"spec,omitempty"`
	Status SecurityGroupStatus `json:"status,omitempty"`
}

// SecurityGroupList contains a list of SecurityGroup
// +kubebuilder:object:root=true
type SecurityGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecurityGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SecurityGroup{}, &SecurityGroupList{})
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecurityGroupSpec) DeepCopyInto(out *SecurityGroupSpec) {
	*out = *in
	if in.Region != nil {
		in, out := &in.Region, &out.Region
		*out = new(string)
		**out = **in
	}
	if in.SecurityGroup != nil {
		in, out := &in.SecurityGroup, &out.SecurityGroup
		*out = new(tcvpc.SecurityGroup)
		**out = **in
	}
	if in.SecurityGroupPolicySet != nil {
		in, out := &in.SecurityGroupPolicySet, &out.SecurityGroupPolicySet
		*out = new(tcvpc.SecurityGroupPolicySet)
		**out = **in
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SecurityGroupStatus) DeepCopyInto(out *SecurityGroupStatus) {
	*out = *in
	if in.ResourceStatus != nil {
		in, out := &in.ResourceStatus, &out.ResourceStatus
		*out = new(common.ResourceStatus)
		**out = **in
	}
	if in.SecurityGroupStatus != nil {
		in, out := &in.SecurityGroupStatus, &out.SecurityGroupStatus
		*out = new(tcvpc.SecurityGroup)
		**out = **in
	}
	if in.SecurityGroupPolicySetStatus != nil {
		in, out := &in.SecurityGroupPolicySetStatus, &out.SecurityGroupPolicySetStatus
		*out = new(tcvpc.SecurityGroupPolicySet)
		**out = **in
	}
}
