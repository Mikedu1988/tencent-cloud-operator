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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	networkv1alpha1 "tencent-cloud-operator/apis/network/v1alpha1"
	"tencent-cloud-operator/internal/common"
	"tencent-cloud-operator/internal/utils"

	log "github.com/sirupsen/logrus"
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tctke "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20180525"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	computev1alpha1 "tencent-cloud-operator/apis/compute/v1alpha1"
)

// NodePoolReconciler reconciles a NodePool object
type NodePoolReconciler struct {
	client.Client
	Log    *log.Logger
	Scheme *runtime.Scheme
}

//Reconcile start reconcile loop
// +kubebuilder:rbac:groups=compute.tencentcloud.kubecooler.com,resources=nodepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compute.tencentcloud.kubecooler.com,resources=nodepools/status,verbs=get;update;patch
func (r *NodePoolReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	r.Log.Info("nodepool", req.NamespacedName)

	// your logic here
	nodePool := &computev1alpha1.NodePool{}
	err := r.Get(ctx, req.NamespacedName, nodePool)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Infof("Request object not found, could have been deleted after reconcile request.")
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		r.Log.Info("error reading the object, requeue")
		return ctrl.Result{}, err
	}
	r.Log.Info("found the cluster", *nodePool.Spec.Name)
	if nodePool.Status.ResourceStatus == nil {
		nodePool.Status.ResourceStatus = new(common.ResourceStatus)
		nodePool.Status.ResourceStatus.Status = new(string)
		nodePool.Status.ResourceStatus.Reason = new(string)
		nodePool.Status.ResourceStatus.RetryCount = new(int)
		nodePool.Status.ResourceStatus.Code = new(string)
		nodePool.Status.ResourceStatus.LastRetry = new(string)
	}
	if nodePool.Status.Cluster == nil {
		nodePool.Status.Cluster = new(tctke.Cluster)
	}
	if nodePool.Status.NodePool == nil {
		nodePool.Status.NodePool = new(tctke.NodePool)
	}
	if nodePool.Status.Vpc == nil {
		nodePool.Status.Vpc = new(tcvpc.Vpc)
		nodePool.Status.Vpc.VpcId = new(string)
	}
	err = r.nodePoolReconcile(nodePool)
	if err != nil {
		if _, ok := err.(*common.ReferencedResourceNotReady); ok {
			r.Log.Info("ref resource not ready, requeue after 30 seconds")
			*nodePool.Status.ResourceStatus.Status = "PENDING"
			*nodePool.Status.ResourceStatus.Reason = "ref resource not ready"
			_ = r.Update(context.TODO(), nodePool)
			return ctrl.Result{RequeueAfter: common.RequeueInterval}, nil
		}
		*nodePool.Status.ResourceStatus.Status = "ERROR"
		*nodePool.Status.ResourceStatus.LastRetry = time.Now().Format("2006-01-02T15:04:05")
		*nodePool.Status.ResourceStatus.RetryCount++
		if cloudError, ok := err.(*tcerrors.TencentCloudSDKError); ok {
			*nodePool.Status.ResourceStatus.Code = cloudError.Code
			*nodePool.Status.ResourceStatus.Reason = cloudError.Message
		}
		_ = r.Update(context.TODO(), nodePool)
		return ctrl.Result{RequeueAfter: common.RequeueInterval}, err
	}
	return ctrl.Result{RequeueAfter: common.RequeueInterval}, nil
}

func (r *NodePoolReconciler) nodePoolReconcile(nodePool *computev1alpha1.NodePool) error {
	// always check for finalizers
	deleted := !nodePool.GetDeletionTimestamp().IsZero()
	pendingFinalizers := nodePool.GetFinalizers()
	finalizerExists := len(pendingFinalizers) > 0
	if !finalizerExists && !deleted && !utils.Contains(pendingFinalizers, common.Finalizer) {
		r.Log.Info("Adding finalized &s to resource", common.Finalizer)
		finalizers := append(pendingFinalizers, common.Finalizer)
		nodePool.SetFinalizers(finalizers)
		err := r.Update(context.TODO(), nodePool)
		if err != nil {
			return err
		}
	}

	//if the resource is not created yet, try to create. processing status will only happen once
	if nodePool.Status.ResourceStatus.Status == nil || *nodePool.Status.ResourceStatus.Status == "" {
		*nodePool.Status.ResourceStatus.Status = "PROCESSING"
		return r.Update(context.TODO(), nodePool)

	}
	if strings.EqualFold(*nodePool.Status.ResourceStatus.Status, "PROCESSING") {
		return r.createNodePool(nodePool)
	}
	r.Log.Infof("nodePool %s is in %s status", *nodePool.Spec.Name, *nodePool.Status.ResourceStatus.Status)

	tencentNodePool, err := r.getNodePool(nodePool)
	if err != nil && !deleted {
		r.Log.Infof("error retrieve nodepool %s status from tencent cloud, just requeue for retry", *nodePool.Spec.Name)
		return err
	}
	if deleted {
		if !strings.EqualFold(*nodePool.Status.ResourceStatus.Status, "DELETING") {
			*nodePool.Status.ResourceStatus.Status = "DELETING"
			return r.Update(context.TODO(), nodePool)
		}
		// resource is marked to be deleted, cloud resource still exists
		if tencentNodePool != nil {
			err := r.deleteNodePool(nodePool)
			if err != nil {
				r.Log.Error(err, "error delete node pool", "namespace:", nodePool.Namespace, "name:", *nodePool.Spec.Name)
				//error delete the resource from cloud, don't remove finalizer yet
				return nil
			}
		}
		// resource deleted from cloud, remove finalizer
		finalizers := make([]string, 0)
		pendingFinalizers = nodePool.GetFinalizers()
		for _, pendingFinalizer := range pendingFinalizers {
			if pendingFinalizer != common.Finalizer {
				finalizers = append(finalizers, pendingFinalizer)
			}
		}
		nodePool.SetFinalizers(finalizers)
		return r.Update(context.TODO(), nodePool)
	}
	if strings.EqualFold(*nodePool.Status.ResourceStatus.Status, "PENDING") || strings.EqualFold(*nodePool.Status.ResourceStatus.Status, "ERROR") {
		r.Log.Infof("nodePool %s is %s in  status, retry", *nodePool.Spec.Name, *nodePool.Status.ResourceStatus.Status)
		lastRetried, _ := time.Parse("2006-01-02T15:04:05", *nodePool.Status.ResourceStatus.LastRetry)
		//only retry 10 times, only retry every 1 minute
		if *nodePool.Status.ResourceStatus.RetryCount < 10 && time.Since(lastRetried) > time.Minute {
			if nodePool.Status.Cluster.ClusterId == nil || *nodePool.Status.Cluster.ClusterId == "" {
				r.Log.Infof("nodePool %s is in %s status, retry create", *nodePool.Spec.Name, *nodePool.Status.ResourceStatus.Status)
				return r.createNodePool(nodePool)
			}
		}
	}
	//update the status
	if !reflect.DeepEqual(nodePool.Status.NodePool, tencentNodePool) {
		r.Log.Info("nodePool not deep equal")
		nodePool.Status.NodePool = tencentNodePool
		//the status could be null
		if tencentNodePool.AutoscalingGroupStatus == nil {
			*nodePool.Status.ResourceStatus.Status = "ERROR"
		} else {
			nodePool.Status.ResourceStatus.Status = tencentNodePool.AutoscalingGroupStatus
		}
		*nodePool.Status.ResourceStatus.RetryCount = 0
		*nodePool.Status.ResourceStatus.LastRetry = ""
		*nodePool.Status.ResourceStatus.Code = ""
		*nodePool.Status.ResourceStatus.Reason = ""
		return r.Update(context.TODO(), nodePool)
	}
	os.LookupEnv("123")
	return nil
}

func (r *NodePoolReconciler) createNodePool(nodePool *computev1alpha1.NodePool) error {
	tencentClient, _ := tctke.NewClient(common.GerCredential(), *nodePool.Spec.Region, profile.NewClientProfile())
	vpcRequest := types.NamespacedName{
		Name:      *nodePool.Spec.VpcRef.Name,
		Namespace: nodePool.Namespace,
	}
	vpc := &networkv1alpha1.Vpc{}
	err := r.Get(context.TODO(), vpcRequest, vpc)
	if err != nil {
		r.Log.Info("can't find vpc ref", "namespace", nodePool.Namespace, "vpcname", nodePool.Spec.VpcRef.Name)
		return err
	}
	if *vpc.Status.ResourceStatus.Status != "READY" {
		r.Log.Info("referenced vpc is not in READY status", "namespace", nodePool.Namespace, "vpcname", nodePool.Spec.VpcRef.Name, "vpc status", vpc.Status.ResourceStatus.Status)
		return &common.ReferencedResourceNotReady{
			Message: fmt.Sprintf("referenced vpc is not in READY status. namespace %s, vpcname: %s, vpc status: %s", nodePool.Namespace, *nodePool.Spec.VpcRef.Name, *vpc.Status.ResourceStatus.Status),
		}
	}
	cluster := &computev1alpha1.ManagedCluster{}
	clusterRequest := types.NamespacedName{
		Name:      *nodePool.Spec.ClusterRef.Name,
		Namespace: nodePool.Namespace,
	}
	err = r.Get(context.TODO(), clusterRequest, cluster)
	if err != nil {
		r.Log.Info("can't find managed cluster ref", "namespace", nodePool.Namespace, "managed cluster name", nodePool.Spec.ClusterRef.Name)
		return err
	}
	if !strings.EqualFold(*cluster.Status.ResourceStatus.Status, "RUNNING") {
		r.Log.Info("referenced managed cluster is not in READY status", "namespace", nodePool.Namespace, "managed cluster name", nodePool.Spec.ClusterRef.Name, "vpc status", cluster.Status.ResourceStatus.Status)
		return &common.ReferencedResourceNotReady{
			Message: fmt.Sprintf("referenced managed cluster is not in READY status. namespace %s, managed cluster name: %s, managed cluster status: %s", nodePool.Namespace, *nodePool.Spec.ClusterRef.Name, *cluster.Status.ResourceStatus.Status),
		}
	}
	request := tctke.NewCreateClusterNodePoolRequest()
	request.Labels = make([]*tctke.Label, 0)
	request.Taints = make([]*tctke.Taint, 0)
	request.AutoScalingGroupPara = new(string)
	request.LaunchConfigurePara = new(string)
	request.ClusterId = cluster.Status.Cluster.ClusterId
	request.Labels = nodePool.Spec.Labels
	request.Taints = nodePool.Spec.Taints
	request.Name = nodePool.Spec.Name
	request.AutoScalingGroupPara, request.LaunchConfigurePara, err = r.getConfigPraStr(nodePool)
	if err != nil {
		return err
	}
	request.EnableAutoscale = nodePool.Spec.EnableAutoScale
	request.OsCustomizeType = nodePool.Spec.OsCustomizeType
	request.NodePoolOs = nodePool.Spec.NodePoolOs

	//required parameter
	request.InstanceAdvancedSettings = new(tctke.InstanceAdvancedSettings)
	resp, err := tencentClient.CreateClusterNodePool(request)
	if err != nil {
		r.Log.Info("error create tencent cloud Managed cluster node pool, err:", err)
		return err
	}
	nodePool.Status.NodePool.NodePoolId = resp.Response.NodePoolId
	nodePool.Status.Cluster.ClusterId = cluster.Status.Cluster.ClusterId
	nodePool.Status.Vpc.VpcId = vpc.Status.VpcID
	*nodePool.Status.ResourceStatus.Status = "CREATING"
	err = r.Update(context.TODO(), nodePool)
	if err != nil {
		r.Log.Info("error update managedCluster status")
		return err
	}
	return nil
}

func (r *NodePoolReconciler) getNodePool(nodePool *computev1alpha1.NodePool) (*tctke.NodePool, error) {
	//cloud resource is not created, return nil
	if nodePool.Status.NodePool.NodePoolId == nil || *nodePool.Status.NodePool.NodePoolId == "" {
		return nil, nil
	}
	tencentClient, _ := tctke.NewClient(common.GerCredential(), *nodePool.Spec.Region, profile.NewClientProfile())
	request := tctke.NewDescribeClusterNodePoolDetailRequest()
	request.ClusterId = nodePool.Status.Cluster.ClusterId
	request.NodePoolId = nodePool.Status.NodePool.NodePoolId
	resp, err := tencentClient.DescribeClusterNodePoolDetail(request)
	if err != nil {
		//TODO: update error handling when tencect cloud have clear error for this scenario
		if cloudError, ok := err.(*tcerrors.TencentCloudSDKError); ok && (strings.Contains(cloudError.Message, "[E501001 DBRecordNotFound] record not found") || strings.Contains(cloudError.Message, "GET_CLUSTER_INFO_ERROR")) {
			r.Log.Info("node pool deleted from cloud, ignore this error")
			return nil, nil
		}
		return nil, err
	}
	return resp.Response.NodePool, nil
}

func (r *NodePoolReconciler) deleteNodePool(nodePool *computev1alpha1.NodePool) error {
	tencentClient, _ := tctke.NewClient(common.GerCredential(), *nodePool.Spec.Region, profile.NewClientProfile())
	request := tctke.NewDeleteClusterNodePoolRequest()
	request.ClusterId = nodePool.Status.Cluster.ClusterId
	request.KeepInstance = new(bool)
	*request.KeepInstance = false
	request.NodePoolIds = append(request.NodePoolIds, nodePool.Status.NodePool.NodePoolId)
	_, err := tencentClient.DeleteClusterNodePool(request)
	if err != nil {
		return err
	}
	return nil
}

func (r *NodePoolReconciler) getConfigPraStr(nodePool *computev1alpha1.NodePool) (autoScaleGroupPara *string, launchConfigPara *string, err error) {
	vpc := &networkv1alpha1.Vpc{}
	vpcRequest := types.NamespacedName{
		Name:      *nodePool.Spec.VpcRef.Name,
		Namespace: nodePool.Namespace,
	}
	err = r.Get(context.TODO(), vpcRequest, vpc)
	if err != nil {
		r.Log.Info("can't find vpc ref", "namespace", nodePool.Namespace, "vpcname", *nodePool.Spec.VpcRef.Name)
		return nil, nil, &common.ReferencedResourceNotReady{Message: fmt.Sprintf("can't find vpc ref in namespace %s, vpc name: %s", nodePool.Namespace, *nodePool.Spec.VpcRef.Name)}
	}
	if *vpc.Status.ResourceStatus.Status != "READY" {
		return nil, nil, &common.ReferencedResourceNotReady{Message: fmt.Sprintf("can't find vpc ref in namespace %s, vpc name: %s", nodePool.Namespace, *nodePool.Spec.VpcRef.Name)}
	}
	autoScaleConfig := nodePool.Spec.AutoScalingConfig.DeepCopy()
	autoScaleConfig.VpcID = vpc.Status.VpcID
	for _, subnetRef := range nodePool.Spec.AutoScalingConfig.SubnetRef {
		subnetRequest := types.NamespacedName{
			Name:      *subnetRef.Name,
			Namespace: nodePool.Namespace,
		}
		subnet := &networkv1alpha1.Subnet{}
		err := r.Get(context.TODO(), subnetRequest, subnet)
		if err != nil {
			r.Log.Info("can't find subnet ref", "namespace", nodePool.Namespace, "subnetname", subnetRef.Name)
			return nil, nil, &common.ReferencedResourceNotReady{Message: fmt.Sprintf("can't find subnet ref in namespace %s, subnet name: %s", nodePool.Namespace, *nodePool.Spec.VpcRef.Name)}
		}
		if *subnet.Status.ResourceStatus.Status != "READY" {
			return nil, nil, &common.ReferencedResourceNotReady{Message: fmt.Sprintf("can't find subnet ref in namespace %s, subnet name: %s", nodePool.Namespace, *nodePool.Spec.VpcRef.Name)}
		}
		autoScaleConfig.SubnetIds = append(autoScaleConfig.SubnetIds, subnet.Status.Subnet.SubnetId)
	}
	autoScaleConfig.SubnetRef = nil
	autoScaleParaStr, _ := json.Marshal(autoScaleConfig)
	launchConfig := nodePool.Spec.LaunchConfig.DeepCopy()
	for _, securityGroupRef := range nodePool.Spec.LaunchConfig.SecurityGroupRef {
		securityGroupRequest := types.NamespacedName{
			Name:      *securityGroupRef.Name,
			Namespace: nodePool.Namespace,
		}
		securityGroup := &networkv1alpha1.SecurityGroup{}
		err := r.Get(context.TODO(), securityGroupRequest, securityGroup)
		if err != nil {
			r.Log.Info("can't find subnet ref", "namespace", nodePool.Namespace, "vpcname", securityGroupRef.Name)
			return nil, nil, &common.ReferencedResourceNotReady{Message: fmt.Sprintf("can't find subnet ref in namespace %s, security group name: %s", nodePool.Namespace, *securityGroupRef.Name)}
		}
		if *securityGroup.Status.ResourceStatus.Status != "READY" {
			return nil, nil, &common.ReferencedResourceNotReady{Message: fmt.Sprintf("security group ref is not ready. namespace %s, security group name: %s", nodePool.Namespace, *securityGroupRef.Name)}
		}
		launchConfig.SecurityGroupIds = append(launchConfig.SecurityGroupIds, securityGroup.Status.SecurityGroupStatus.SecurityGroupId)
	}
	launchConfig.SecurityGroupRef = nil
	launchConfigParaStr, _ := json.Marshal(launchConfig)
	autoScaleGroupPara = new(string)
	launchConfigPara = new(string)
	*autoScaleGroupPara = string(autoScaleParaStr)
	*launchConfigPara = string(launchConfigParaStr)
	return autoScaleGroupPara, launchConfigPara, nil
}

//SetupWithManager setup controller with manager
func (r *NodePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1alpha1.NodePool{}).
		Complete(r)
}
