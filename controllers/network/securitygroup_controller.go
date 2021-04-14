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
	"log"
	"strings"
	"tencent-cloud-operator/internal/tencent/common"
	"time"

	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1alpha1 "tencent-cloud-operator/apis/network/v1alpha1"
)

// SecurityGroupReconciler reconciles a SecurityGroup object
type SecurityGroupReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=network.tencentcloud.kubecooler.com,resources=securitygroups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.tencentcloud.kubecooler.com,resources=securitygroups/status,verbs=get;update;patch

func (r *SecurityGroupReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("securitygroup", req.NamespacedName)

	// your logic here
	securityGroup := &networkv1alpha1.SecurityGroup{}
	err := r.Get(ctx, req.NamespacedName, securityGroup)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Printf("Request object not found, could have been deleted after reconcile request.")
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		log.Println("error reading the object, requeue")
		return ctrl.Result{}, err
	}

	log.Println("found the securityGroup", *securityGroup.Spec.SecurityGroup.SecurityGroupName)
	if securityGroup.Status.Status == nil {
		securityGroup.Status.Status = new(string)
	}
	if securityGroup.Status.LastRetry == nil {
		securityGroup.Status.LastRetry = new(string)
	}
	if securityGroup.Status.Reason == nil {
		securityGroup.Status.Reason = new(string)
	}
	if securityGroup.Status.RetryCount == nil {
		securityGroup.Status.RetryCount = new(int)
	}
	if securityGroup.Status.Code == nil {
		securityGroup.Status.Code = new(string)
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
		*securityGroup.Status.Status = "ERROR"
		*securityGroup.Status.LastRetry = time.Now().Format("2006-01-02T15:04:05")
		*securityGroup.Status.RetryCount++
		if cloudError, ok := err.(*tcerrors.TencentCloudSDKError); ok {
			*securityGroup.Status.Code = cloudError.Code
			*securityGroup.Status.Reason = cloudError.Message
		}
		_ = r.Update(context.TODO(), securityGroup)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *SecurityGroupReconciler) securityGroupReconcile(securityGroup *networkv1alpha1.SecurityGroup) error {
	if securityGroup.Status.Status == nil || *securityGroup.Status.Status == "" {
		*securityGroup.Status.Status = "PROCESSING"
		_ = r.Update(context.TODO(), securityGroup)
		return r.createSecurityGroup(securityGroup)
	} else if *securityGroup.Status.Status == "PROCESSING" {
		log.Printf("SecurityGroup %s is in PROCESSING status, ignore", *securityGroup.Spec.SecurityGroup.SecurityGroupName)
	} else if *securityGroup.Status.Status == "PENDING" {
		log.Printf("SecurityGroup %s is in PENDING status, ignore for now", *securityGroup.Spec.SecurityGroup.SecurityGroupName)
	} else if *securityGroup.Status.Status == "ERROR" {
		lastRetried, _ := time.Parse("2006-01-02T15:04:05", *securityGroup.Status.LastRetry)
		//only retry 10 times, only retry every 1 minute
		if *securityGroup.Status.RetryCount < 10 && time.Since(lastRetried) > time.Minute {
			if securityGroup.Status.SecurityGroupStatus.SecurityGroupId == nil || *securityGroup.Status.SecurityGroupStatus.SecurityGroupId == "" {
				return r.createSecurityGroup(securityGroup)
			} else {
				return r.checkSecurityGroupStatus(securityGroup)
			}
		}
		log.Println("not retrying for SecurityGroup:", *securityGroup.Spec.SecurityGroup.SecurityGroupName)
	} else if *securityGroup.Status.Status == "READY" {
		log.Printf("SecurityGroup %s is in READY status, check the status from tencnet cloud", *securityGroup.Spec.SecurityGroup.SecurityGroupName)
		return r.checkSecurityGroupStatus(securityGroup)
	}
	return nil
}

func (r *SecurityGroupReconciler) createSecurityGroup(securityGroup *networkv1alpha1.SecurityGroup) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *securityGroup.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewCreateSecurityGroupRequest()
	request.GroupName = securityGroup.Spec.SecurityGroup.SecurityGroupName
	request.GroupDescription = securityGroup.Spec.SecurityGroup.SecurityGroupDesc
	request.Tags = securityGroup.Spec.SecurityGroup.TagSet
	log.Println("request:", request.ToJsonString())
	resp, err := tencentClient.CreateSecurityGroup(request)
	if err != nil {
		log.Println("error create tencent cloud SecurityGroup, err:", err)
		return err
	}
	securityGroup.Status.SecurityGroupStatus.SecurityGroupId = resp.Response.SecurityGroup.SecurityGroupId
	err = r.Update(context.TODO(), securityGroup)
	if err != nil {
		log.Println("error update vpc status")
		return err
	}
	//this is the first time we create the security group, just create all group policy set
	err = r.syncSecurityGroupPolicySet(securityGroup)
	if err != nil {
		log.Println("error create tencent cloud SecurityGroupPolicySet, err:", err)
		return err
	}
	*securityGroup.Status.Status = "READY"
	err = r.Update(context.TODO(), securityGroup)
	if err != nil {
		log.Println("error update vpc status")
		return err
	}
	return nil
}

func (r *SecurityGroupReconciler) checkSecurityGroupStatus(securityGroup *networkv1alpha1.SecurityGroup) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *securityGroup.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewDescribeSecurityGroupsRequest()
	request.SecurityGroupIds = append(request.SecurityGroupIds, securityGroup.Status.SecurityGroupStatus.SecurityGroupId)
	resp, err := tencentClient.DescribeSecurityGroups(request)
	if err != nil {
		log.Println("failed to get subnet from tencent cloud, requeue")
		*securityGroup.Status.RetryCount++
		*securityGroup.Status.Reason = err.Error()
		if *securityGroup.Status.RetryCount > 10 {
			return err
		}
		return nil
	}
	if *resp.Response.TotalCount == 0 {
		log.Println("Resource is deleted from cloud, update status")
		*securityGroup.Status.Status = "DELETED_IN_CLOUD"
		err := r.Update(context.Background(), securityGroup)
		if err != nil {
			log.Println("error update DELETED_IN_CLOUD")
		}
	}
	err = r.syncSecurityGroupPolicySet(securityGroup)
	if err != nil {
		log.Println("failed to check security group policy status from tencent cloud, requeue")
		*securityGroup.Status.RetryCount++
		*securityGroup.Status.Reason = err.Error()
		if *securityGroup.Status.RetryCount > 10 {
			return err
		}
		return nil
	}
	*securityGroup.Status.RetryCount = 0
	*securityGroup.Status.LastRetry = ""
	*securityGroup.Status.Code = ""
	*securityGroup.Status.Reason = ""
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
	insertPolicyRequest := tcvpc.NewCreateSecurityGroupPoliciesRequest()
	insertPolicyRequest.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
	insertPolicyRequest.SecurityGroupPolicySet = new(tcvpc.SecurityGroupPolicySet)
	replacePolicyRequest := tcvpc.NewReplaceSecurityGroupPolicyRequest()
	replacePolicyRequest.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
	replacePolicyRequest.SecurityGroupPolicySet = new(tcvpc.SecurityGroupPolicySet)
	deletePolicyRequest := tcvpc.NewDeleteSecurityGroupPoliciesRequest()
	deletePolicyRequest.SecurityGroupId = securityGroup.Status.SecurityGroupStatus.SecurityGroupId
	deletePolicyRequest.SecurityGroupPolicySet = new(tcvpc.SecurityGroupPolicySet)
	insertPolicySet, replacePolicySet, deletePolicySet := diffSecurityGroupPolicySet(resp.Response.SecurityGroupPolicySet, securityGroup.Spec.SecurityGroupPolicySet)
	if len(replacePolicySet.Ingress) != 0 || len(replacePolicySet.Egress) != 0 {
		replacePolicyRequest.SecurityGroupPolicySet = replacePolicySet
		_, err := tencentClient.ReplaceSecurityGroupPolicy(replacePolicyRequest)
		if err != nil {
			return err
		}
	}
	if len(insertPolicySet.Ingress) != 0 || len(insertPolicySet.Egress) != 0 {
		insertPolicyRequest.SecurityGroupPolicySet = insertPolicySet
		_, err := tencentClient.CreateSecurityGroupPolicies(insertPolicyRequest)
		if err != nil {
			return err
		}
	}
	if len(deletePolicySet.Ingress) != 0 || len(deletePolicySet.Egress) != 0 {
		deletePolicyRequest.SecurityGroupPolicySet = deletePolicySet
		_, err := tencentClient.DeleteSecurityGroupPolicies(deletePolicyRequest)
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
		if strings.EqualFold(*policy.Ingress[index].CidrBlock, *ingress.CidrBlock) || strings.EqualFold(*policy.Ingress[index].Port, *ingress.Port) || strings.EqualFold(*policy.Ingress[index].Action, *ingress.Action) || strings.EqualFold(*policy.Ingress[index].Protocol, *ingress.Protocol) || *policy.Ingress[index].PolicyDescription != *ingress.PolicyDescription {
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
		if strings.EqualFold(*policy.Egress[index].CidrBlock, *egress.CidrBlock) || strings.EqualFold(*policy.Egress[index].Port, *egress.Port) || strings.EqualFold(*policy.Egress[index].Action, *egress.Action) || strings.EqualFold(*policy.Egress[index].Protocol, *egress.Protocol) || *policy.Egress[index].PolicyDescription != *egress.PolicyDescription {
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
