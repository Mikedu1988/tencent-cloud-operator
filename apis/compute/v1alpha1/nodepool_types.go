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
	tccvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	tctke "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20180525"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"tencent-cloud-operator/internal/common"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodePoolSpec defines the desired state of NodePool
type NodePoolSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Region            *string           `json:"region,omitempty"`
	Name              *string           `json:"name,omitempty"`
	EnableAutoScale   *bool             `json:"enableAutoScale,omitempty"`
	NodePoolOs        *string           `json:"nodePoolOs,omitempty"`
	OsCustomizeType   *string           `json:"osCustomizeType,omitempty"`
	AutoScalingConfig *AutoScalingGroup `json:"autoScalingConfig,omitempty"`
	LaunchConfig      *LaunchConfig     `json:"launchConfig,omitempty"`
	ClusterRef        *ResourceRef      `json:"clusterRef,omitempty"`
	VpcRef            *ResourceRef      `json:"vpcRef,omitempty"`
	Labels            []*tctke.Label    `json:"labels,omitempty"`
	Taints            []*tctke.Taint    `json:"taints,omitempty"`
}

// AutoScalingGroup autoScalingGroup is a group of kubernetes node
type AutoScalingGroup struct {
	DesiredCapacity *int           `json:"DesiredCapacity,omitempty"`
	MaxSize         *int           `json:"MaxSize,omitempty"`
	MinSize         *int           `json:"MinSize,omitempty"`
	SubnetRef       []*ResourceRef `json:"SubnetRef,omitempty"`
	RetryPolicy     *string        `json:"RetryPolicy,omitempty"`
	SubnetIds       []*string      `json:"SubnetIds,omitempty"`
	VpcID           *string        `json:"VpcId,omitempty"`
}

// LaunchConfig launchConfig for tencent cloud node pool
type LaunchConfig struct {
	InstanceType       *string                   `json:"InstanceType,omitempty"`
	SystemDisk         *tccvm.SystemDisk         `json:"SystemDisk,omitempty"`
	SecurityGroupRef   []*ResourceRef            `json:"SecurityGroupRef,omitempty"`
	SecurityGroupIds   []*string                 `json:"SecurityGroupIds,omitempty"`
	InternetAccessible *tccvm.InternetAccessible `json:"InternetAccessible,omitempty"`
	InstanceChargeType *string                   `json:"InstanceChargeType,omitempty"`
	LoginSettings      *tctke.LoginSettings      `json:"LoginSettings,omitempty"`
	EnhancedService    *tccvm.EnhancedService    `json:"EnhancedService,omitempty"`
}

// ResourceRef defines the resource ref, not allow to refer from another namespace
type ResourceRef struct {
	Name *string `json:"name,omitempty"`
}

// NodePoolStatus defines the observed state of NodePool
type NodePoolStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ResourceStatus *common.ResourceStatus `json:"resourceStatus,omitempty"`
	NodePool       *tctke.NodePool        `json:"nodePool,omitempty"`
	Cluster        *tctke.Cluster         `json:"cluster,omitempty"`
	Vpc            *tcvpc.Vpc             `json:"vpc,omitempty"`
}

// NodePool is the Schema for the nodepools API
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="NodePool_Name",type=string,JSONPath=`.status.nodePool.Name`
// +kubebuilder:printcolumn:name="Node_Type",type=string,JSONPath=`.spec.launchConfig.InstanceType`
// +kubebuilder:printcolumn:name="Desired_Nodes",type=string,JSONPath=`.status.nodePool.DesiredNodesNum`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.resourceStatus.status`
// +kubebuilder:printcolumn:name="Taints",type=string,JSONPath=`.spec.taints`
type NodePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodePoolSpec   `json:"spec,omitempty"`
	Status NodePoolStatus `json:"status,omitempty"`
}

// NodePoolList contains a list of NodePool
// +kubebuilder:object:root=true
type NodePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodePool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodePool{}, &NodePoolList{})
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LaunchConfig) DeepCopyInto(out *LaunchConfig) {
	*out = *in
	if in.InstanceType != nil {
		in, out := &in.InstanceType, &out.InstanceType
		*out = new(string)
		**out = **in
	}
	if in.SystemDisk != nil {
		in, out := &in.SystemDisk, &out.SystemDisk
		*out = new(tccvm.SystemDisk)
		**out = **in
	}
	if in.SecurityGroupRef != nil {
		in, out := &in.SecurityGroupRef, &out.SecurityGroupRef
		*out = make([]*ResourceRef, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(ResourceRef)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.SecurityGroupIds != nil {
		in, out := &in.SecurityGroupIds, &out.SecurityGroupIds
		*out = make([]*string, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(string)
				**out = **in
			}
		}
	}
	if in.InstanceChargeType != nil {
		in, out := &in.InstanceChargeType, &out.InstanceChargeType
		*out = new(string)
		**out = **in
	}
	if in.InternetAccessible != nil {
		in, out := &in.InternetAccessible, &out.InternetAccessible
		*out = new(tccvm.InternetAccessible)
		**out = **in
	}
	if in.LoginSettings != nil {
		in, out := &in.LoginSettings, &out.LoginSettings
		*out = new(tctke.LoginSettings)
		**out = **in
	}
	if in.EnhancedService != nil {
		in, out := &in.EnhancedService, &out.EnhancedService
		*out = new(tccvm.EnhancedService)
		**out = **in
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodePoolSpec) DeepCopyInto(out *NodePoolSpec) {
	*out = *in
	if in.Region != nil {
		in, out := &in.Region, &out.Region
		*out = new(string)
		**out = **in
	}
	if in.Name != nil {
		in, out := &in.Name, &out.Name
		*out = new(string)
		**out = **in
	}
	if in.EnableAutoScale != nil {
		in, out := &in.EnableAutoScale, &out.EnableAutoScale
		*out = new(bool)
		**out = **in
	}
	if in.NodePoolOs != nil {
		in, out := &in.NodePoolOs, &out.NodePoolOs
		*out = new(string)
		**out = **in
	}
	if in.OsCustomizeType != nil {
		in, out := &in.OsCustomizeType, &out.OsCustomizeType
		*out = new(string)
		**out = **in
	}
	if in.AutoScalingConfig != nil {
		in, out := &in.AutoScalingConfig, &out.AutoScalingConfig
		*out = new(AutoScalingGroup)
		(*in).DeepCopyInto(*out)
	}
	if in.LaunchConfig != nil {
		in, out := &in.LaunchConfig, &out.LaunchConfig
		*out = new(LaunchConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.ClusterRef != nil {
		in, out := &in.ClusterRef, &out.ClusterRef
		*out = new(ResourceRef)
		(*in).DeepCopyInto(*out)
	}
	if in.VpcRef != nil {
		in, out := &in.VpcRef, &out.VpcRef
		*out = new(ResourceRef)
		(*in).DeepCopyInto(*out)
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make([]*tctke.Label, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(tctke.Label)
				**out = **in
			}
		}
	}
	if in.Taints != nil {
		in, out := &in.Taints, &out.Taints
		*out = make([]*tctke.Taint, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(tctke.Taint)
				**out = **in
			}
		}
	}
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NodePoolStatus) DeepCopyInto(out *NodePoolStatus) {
	*out = *in
	if in.ResourceStatus != nil {
		in, out := &in.ResourceStatus, &out.ResourceStatus
		*out = new(common.ResourceStatus)
		**out = **in
	}
	if in.NodePool != nil {
		in, out := &in.NodePool, &out.NodePool
		*out = new(tctke.NodePool)
		**out = **in
	}
	if in.Cluster != nil {
		in, out := &in.Cluster, &out.Cluster
		*out = new(tctke.Cluster)
		**out = **in
	}
	if in.Vpc != nil {
		in, out := &in.Vpc, &out.Vpc
		*out = new(tcvpc.Vpc)
		**out = **in
	}
}
