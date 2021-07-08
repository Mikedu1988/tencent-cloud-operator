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
	"reflect"
	"strings"
	"time"

	"tencent-cloud-operator/internal/common"
	"tencent-cloud-operator/internal/utils"

	log "github.com/sirupsen/logrus"
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1alpha1 "tencent-cloud-operator/apis/network/v1alpha1"
)

// SecurityGroupReconciler reconciles a SecurityGroup object
type SecurityGroupReconciler struct {
	client.Client
	Log    *log.Logger
	Scheme *runtime.Scheme
}

// Reconcile start reconcile loop
// +kubebuilder:rbac:groups=network.tencentcloud.kubecooler.com,resources=securitygroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.tencentcloud.kubecooler.com,resources=securitygroups/status,verbs=get;update;patch
func (r *SecurityGroupReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	r.Log.Info("securitygroup", req.NamespacedName)

	// your logic here
	securityGroup := &networkv1alpha1.SecurityGroup{}
	err := r.Get(ctx, req.NamespacedName, securityGroup)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Request object not found, could have been deleted after reconcile request.")
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		r.Log.Info("error reading the object, requeue")
		return ctrl.Result{}, err
	}

	r.Log.Info("found the securityGroup", *securityGroup.Spec.SecurityGroup.SecurityGroupName)
	if securityGroup.Status.ResourceStatus == nil {
		securityGroup.Status.ResourceStatus = new(common.ResourceStatus)
		securityGroup.Status.ResourceStatus.Status = new(string)
		securityGroup.Status.ResourceStatus.Reason = new(string)
		securityGroup.Status.ResourceStatus.RetryCount = new(int)
		securityGroup.Status.ResourceStatus.Code = new(string)
		securityGroup.Status.ResourceStatus.LastRetry = new(string)
	}
	if securityGroup.Status.SecurityGroupStatus == nil {
		securityGroup.Status.SecurityGroupStatus = &tcvpc.SecurityGroup{}
	}
	if securityGroup.Status.SecurityGroupPolicySetStatus == nil {
		securityGroup.Status.SecurityGroupPolicySetStatus = &tcvpc.SecurityGroupPolicySet{
			Version: new(string),
			Ingress: make([]*tcvpc.SecurityGroupPolicy, 0),
			Egress:  make([]*tcvpc.SecurityGroupPolicy, 0),
		}
	}
	err = r.securityGroupReconcile(securityGroup)
	if err != nil {
		*securityGroup.Status.ResourceStatus.Status = "ERROR"
		*securityGroup.Status.ResourceStatus.LastRetry = time.Now().UTC().Format("2006-01-02T15:04:05")
		*securityGroup.Status.ResourceStatus.RetryCount++
		if cloudError, ok := err.(*tcerrors.TencentCloudSDKError); ok {
			*securityGroup.Status.ResourceStatus.Code = cloudError.Code
			*securityGroup.Status.ResourceStatus.Reason = cloudError.Message
		}
		_ = r.Update(context.TODO(), securityGroup)
		return ctrl.Result{RequeueAfter: common.RequeueInterval}, err
	}
	return ctrl.Result{RequeueAfter: common.RequeueInterval}, nil
}

func (r *SecurityGroupReconciler) securityGroupReconcile(securityGroup *networkv1alpha1.SecurityGroup) error {
	// always check for finalizers
	deleted := !securityGroup.GetDeletionTimestamp().IsZero()
	pendingFinalizers := securityGroup.GetFinalizers()
	finalizerExists := len(pendingFinalizers) > 0
	if !finalizerExists && !deleted && !utils.Contains(pendingFinalizers, common.Finalizer) {
		r.Log.Info("Adding finalized &s to resource", common.Finalizer)
		finalizers := append(pendingFinalizers, common.Finalizer)
		securityGroup.SetFinalizers(finalizers)
		err := r.Update(context.TODO(), securityGroup)
		if err != nil {
			return err
		}
	}
	if securityGroup.Status.ResourceStatus.Status == nil || *securityGroup.Status.ResourceStatus.Status == "" {
		*securityGroup.Status.ResourceStatus.Status = "PROCESSING"
		return r.Update(context.TODO(), securityGroup)

	}
	if *securityGroup.Status.ResourceStatus.Status == "PROCESSING" {
		return r.createSecurityGroup(securityGroup)
	}
	r.Log.Infof("subnet %s is in %s status", *securityGroup.Spec.SecurityGroup.SecurityGroupName, *securityGroup.Status.ResourceStatus.Status)
	tencentSecurityGroup, err := r.getSecurityGroup(securityGroup)
	if err != nil {
		r.Log.Infof("error retrieve subnet %s status from tencent cloud, just requeue for retry", *securityGroup.Spec.SecurityGroup.SecurityGroupName)
		return err
	}
	if deleted {
		if !strings.EqualFold(*securityGroup.Status.ResourceStatus.Status, "DELETING") {
			*securityGroup.Status.ResourceStatus.Status = "DELETING"
			return r.Update(context.TODO(), securityGroup)
		}
		// resource is marked to be deleted, cloud resource still exists
		if tencentSecurityGroup != nil {
			err = r.deleteSecurityGroup(securityGroup)
			if err != nil {
				r.Log.Error(err, "error delete security group", "namespace:", securityGroup.Namespace, "name:", *securityGroup.Spec.SecurityGroup.SecurityGroupName)
				//error delete the resource from cloud, don't remove finalizer yet
				return nil
			}
		}
		// resource deleted from cloud, remove finalizer
		finalizers := make([]string, 0)
		pendingFinalizers = securityGroup.GetFinalizers()
		for _, pendingFinalizer := range pendingFinalizers {
			if pendingFinalizer != common.Finalizer {
				finalizers = append(finalizers, pendingFinalizer)
			}
		}
		securityGroup.SetFinalizers(finalizers)
		return r.Update(context.TODO(), securityGroup)
	}
	if strings.EqualFold(*securityGroup.Status.ResourceStatus.Status, "PENDING") || strings.EqualFold(*securityGroup.Status.ResourceStatus.Status, "ERROR") {
		r.Log.Infof("subnet %s is in  status, retry", *securityGroup.Spec.SecurityGroup.SecurityGroupName)
		lastRetried, _ := time.Parse("2006-01-02T15:04:05", *securityGroup.Status.ResourceStatus.LastRetry)
		//only retry 10 times, only retry every 1 minute
		if *securityGroup.Status.ResourceStatus.RetryCount < 10 && time.Since(lastRetried) > time.Minute {
			if securityGroup.Status.SecurityGroupStatus.SecurityGroupId == nil || *securityGroup.Status.SecurityGroupStatus.SecurityGroupId == "" {
				return r.createSecurityGroup(securityGroup)
			}
		}
	}
	// resource deleted in cloud, update the status
	if tencentSecurityGroup == nil {
		if strings.EqualFold(*securityGroup.Status.ResourceStatus.Status, "READY") {
			*securityGroup.Status.ResourceStatus.RetryCount = 0
			*securityGroup.Status.ResourceStatus.LastRetry = ""
			*securityGroup.Status.ResourceStatus.Code = ""
			*securityGroup.Status.ResourceStatus.Reason = "security group deleted in cloud"
			*securityGroup.Status.ResourceStatus.Status = "ERROR"
			return r.Update(context.TODO(), securityGroup)
		}
		return nil
	}
	// resource found in cloud, not marked as deleted, try sync the security group rules first
	err = r.syncSecurityGroupPolicySet(securityGroup)
	if err != nil {
		r.Log.Errorf("failed to sync security group policy status from tencent cloud, requeue")
		//we get the security group from tencent cloud, but we can't sync security group rules , ignore the security group
		return err
	}
	// security group rule sync success, update status
	if !reflect.DeepEqual(securityGroup.Status.SecurityGroupStatus, tencentSecurityGroup) || !strings.EqualFold(*securityGroup.Status.ResourceStatus.Status, "READY") {
		*securityGroup.Status.SecurityGroupStatus = *tencentSecurityGroup
		*securityGroup.Status.ResourceStatus.RetryCount = 0
		*securityGroup.Status.ResourceStatus.LastRetry = ""
		*securityGroup.Status.ResourceStatus.Code = ""
		*securityGroup.Status.ResourceStatus.Reason = ""
		*securityGroup.Status.ResourceStatus.Status = "READY"
		return r.Update(context.TODO(), securityGroup)
	}
	return nil
}

func (r *SecurityGroupReconciler) createSecurityGroup(securityGroup *networkv1alpha1.SecurityGroup) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *securityGroup.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewCreateSecurityGroupRequest()
	request.GroupName = securityGroup.Spec.SecurityGroup.SecurityGroupName
	request.GroupDescription = securityGroup.Spec.SecurityGroup.SecurityGroupDesc
	request.Tags = securityGroup.Spec.SecurityGroup.TagSet
	r.Log.Info("request:", request.ToJsonString())
	resp, err := tencentClient.CreateSecurityGroup(request)
	if err != nil {
		r.Log.Info("error create tencent cloud SecurityGroup, err:", err)
		return err
	}
	securityGroup.Status.SecurityGroupStatus.SecurityGroupId = resp.Response.SecurityGroup.SecurityGroupId
	err = r.Update(context.TODO(), securityGroup)
	if err != nil {
		r.Log.Info("error update vpc status")
		return err
	}
	//this is the first time we create the security group, just create all group policy set
	err = r.syncSecurityGroupPolicySet(securityGroup)
	if err != nil {
		r.Log.Info("error create tencent cloud SecurityGroupPolicySet, err:", err)
		return err
	}
	*securityGroup.Status.ResourceStatus.Status = "READY"
	err = r.Update(context.TODO(), securityGroup)
	if err != nil {
		r.Log.Info("error update vpc status")
		return err
	}
	return nil
}

func (r *SecurityGroupReconciler) getSecurityGroup(securityGroup *networkv1alpha1.SecurityGroup) (*tcvpc.SecurityGroup, error) {
	//cloud resource is not created, return nil
	if securityGroup.Status.SecurityGroupStatus.SecurityGroupId == nil || *securityGroup.Status.SecurityGroupStatus.SecurityGroupId == "" {
		return nil, nil
	}
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *securityGroup.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewDescribeSecurityGroupsRequest()
	request.SecurityGroupIds = append(request.SecurityGroupIds, securityGroup.Status.SecurityGroupStatus.SecurityGroupId)
	resp, err := tencentClient.DescribeSecurityGroups(request)
	if err != nil {
		r.Log.Info("failed to get securityGroup from tencent cloud, requeue")
		return nil, err
	}
	if *resp.Response.TotalCount == 0 {
		r.Log.Info("Resource is deleted from cloud, update status")
		return nil, nil
	}
	return resp.Response.SecurityGroupSet[0], nil
}

func (r *SecurityGroupReconciler) deleteSecurityGroup(securityGroup *networkv1alpha1.SecurityGroup) error {
	if securityGroup.Status.SecurityGroupStatus.SecurityGroupId == nil || *securityGroup.Status.SecurityGroupStatus.SecurityGroupId == "" {
		return nil
	}
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *securityGroup.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewDeleteSecurityGroupRequest()
	request.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
	_, err := tencentClient.DeleteSecurityGroup(request)
	if err != nil {
		r.Log.Info("failed to delete securityGroup from tencent cloud, requeue")
		return err
	}
	return nil
}

func (r *SecurityGroupReconciler) syncSecurityGroupPolicySet(securityGroup *networkv1alpha1.SecurityGroup) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *securityGroup.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewDescribeSecurityGroupPoliciesRequest()
	request.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
	resp, err := tencentClient.DescribeSecurityGroupPolicies(request)
	if err != nil {
		return err
	}
	//sync policy
	insertPolicySet, replacePolicySet, deletePolicySet := diffSecurityGroupPolicySet(resp.Response.SecurityGroupPolicySet, securityGroup.Spec.SecurityGroupPolicySet)
	if len(deletePolicySet.Ingress) != 0 || len(deletePolicySet.Egress) != 0 {
		deletePolicyRequest := tcvpc.NewDeleteSecurityGroupPoliciesRequest()
		deletePolicyRequest.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
		deletePolicyRequest.SecurityGroupPolicySet = new(tcvpc.SecurityGroupPolicySet)
		deletePolicyRequest.SecurityGroupPolicySet = deletePolicySet
		_, err = tencentClient.DeleteSecurityGroupPolicies(deletePolicyRequest)
		if err != nil {
			return err
		}
	}
	if len(insertPolicySet.Ingress) != 0 {
		insertIngressPolicyRequest := tcvpc.NewCreateSecurityGroupPoliciesRequest()
		insertIngressPolicyRequest.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
		insertIngressPolicyRequest.SecurityGroupPolicySet = new(tcvpc.SecurityGroupPolicySet)
		*insertIngressPolicyRequest.SecurityGroupPolicySet = *insertPolicySet
		insertIngressPolicyRequest.SecurityGroupPolicySet.Egress = nil
		_, err = tencentClient.CreateSecurityGroupPolicies(insertIngressPolicyRequest)
		if err != nil {
			return err
		}
	}
	if len(insertPolicySet.Egress) != 0 {
		insertEgressPolicyRequest := tcvpc.NewCreateSecurityGroupPoliciesRequest()
		insertEgressPolicyRequest.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
		insertEgressPolicyRequest.SecurityGroupPolicySet = new(tcvpc.SecurityGroupPolicySet)
		*insertEgressPolicyRequest.SecurityGroupPolicySet = *insertPolicySet
		insertEgressPolicyRequest.SecurityGroupPolicySet.Ingress = nil
		_, err = tencentClient.CreateSecurityGroupPolicies(insertEgressPolicyRequest)
		if err != nil {
			return err
		}
	}
	if len(replacePolicySet.Ingress) != 0 {
		replaceIngressPolicyRequest := tcvpc.NewReplaceSecurityGroupPolicyRequest()
		replaceIngressPolicyRequest.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
		replaceIngressPolicyRequest.SecurityGroupPolicySet = new(tcvpc.SecurityGroupPolicySet)
		replaceIngressPolicyRequest.SecurityGroupPolicySet = replacePolicySet
		replaceIngressPolicyRequest.SecurityGroupPolicySet.Egress = nil
		_, err = tencentClient.ReplaceSecurityGroupPolicy(replaceIngressPolicyRequest)
		if err != nil {
			return err
		}
	}
	if len(replacePolicySet.Egress) != 0 {
		replaceEgressPolicyRequest := tcvpc.NewReplaceSecurityGroupPolicyRequest()
		replaceEgressPolicyRequest.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
		replaceEgressPolicyRequest.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
		replaceEgressPolicyRequest.SecurityGroupPolicySet = new(tcvpc.SecurityGroupPolicySet)
		replaceEgressPolicyRequest.SecurityGroupPolicySet = replacePolicySet
		replaceEgressPolicyRequest.SecurityGroupPolicySet.Ingress = nil
		_, err = tencentClient.ReplaceSecurityGroupPolicy(replaceEgressPolicyRequest)
		if err != nil {
			return err
		}
	}

	//after the policy sync complete, get the policies again
	resp, err = tencentClient.DescribeSecurityGroupPolicies(request)
	if err != nil {
		return err
	}
	securityGroup.Status.SecurityGroupPolicySetStatus = resp.Response.SecurityGroupPolicySet
	_ = r.Update(context.TODO(), securityGroup)
	return nil
}

//SetupWithManager setup controller with manager
func (r *SecurityGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.SecurityGroup{}).
		Complete(r)
}

func diffSecurityGroupPolicySet(policy *tcvpc.SecurityGroupPolicySet, spec *tcvpc.SecurityGroupPolicySet) (replacePolicySet *tcvpc.SecurityGroupPolicySet, insertPolicySet *tcvpc.SecurityGroupPolicySet, deletePolicySet *tcvpc.SecurityGroupPolicySet) {
	replacePolicySet = new(tcvpc.SecurityGroupPolicySet)
	replacePolicySet.Egress = make([]*tcvpc.SecurityGroupPolicy, 0)
	replacePolicySet.Ingress = make([]*tcvpc.SecurityGroupPolicy, 0)
	insertPolicySet = new(tcvpc.SecurityGroupPolicySet)
	insertPolicySet.Egress = make([]*tcvpc.SecurityGroupPolicy, 0)
	insertPolicySet.Ingress = make([]*tcvpc.SecurityGroupPolicy, 0)
	deletePolicySet = new(tcvpc.SecurityGroupPolicySet)
	deletePolicySet.Egress = make([]*tcvpc.SecurityGroupPolicy, 0)
	deletePolicySet.Ingress = make([]*tcvpc.SecurityGroupPolicy, 0)
	for index, ingress := range spec.Ingress {
		//if the spec is longer than the policies we get from cloud, just insert the spec
		if index > len(policy.Ingress)-1 {
			insertPolicySet.Ingress = append(insertPolicySet.Ingress, ingress)
			continue
		}
		//if the policy we get from cloud dose not match spec, replace the policy
		if !strings.EqualFold(*policy.Ingress[index].CidrBlock, *ingress.CidrBlock) || !strings.EqualFold(*policy.Ingress[index].Port, *ingress.Port) || !strings.EqualFold(*policy.Ingress[index].Action, *ingress.Action) || !strings.EqualFold(*policy.Ingress[index].Protocol, *ingress.Protocol) || *policy.Ingress[index].PolicyDescription != *ingress.PolicyDescription {
			ingress.PolicyIndex = new(int64)
			*ingress.PolicyIndex = int64(index)
			replacePolicySet.Ingress = append(replacePolicySet.Ingress, ingress)
			continue
		}
	}
	for index, egress := range spec.Egress {
		if index > len(policy.Egress)-1 {
			insertPolicySet.Egress = append(insertPolicySet.Egress, egress)
			continue
		}
		if !strings.EqualFold(*policy.Egress[index].CidrBlock, *egress.CidrBlock) || !strings.EqualFold(*policy.Egress[index].Port, *egress.Port) || !strings.EqualFold(*policy.Egress[index].Action, *egress.Action) || !strings.EqualFold(*policy.Egress[index].Protocol, *egress.Protocol) || *policy.Egress[index].PolicyDescription != *egress.PolicyDescription {
			egress.PolicyIndex = new(int64)
			*egress.PolicyIndex = int64(index)
			replacePolicySet.Egress = append(replacePolicySet.Egress, egress)
			continue
		}
	}
	//if the cloud policy is longer than spec, delete them
	for i := len(spec.Egress); i < len(policy.Egress); i++ {
		tmpPolicy := new(tcvpc.SecurityGroupPolicy)
		tmpPolicy.PolicyIndex = new(int64)
		*tmpPolicy.PolicyIndex = int64(i)
		deletePolicySet.Egress = append(deletePolicySet.Egress, tmpPolicy)
	}
	for i := len(spec.Ingress); i < len(policy.Ingress); i++ {
		tmpPolicy := new(tcvpc.SecurityGroupPolicy)
		tmpPolicy.PolicyIndex = new(int64)
		*tmpPolicy.PolicyIndex = int64(i)
		deletePolicySet.Ingress = append(deletePolicySet.Ingress, tmpPolicy)
	}
	return insertPolicySet, replacePolicySet, deletePolicySet
}
