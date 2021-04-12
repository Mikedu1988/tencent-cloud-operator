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
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"k8s.io/apimachinery/pkg/api/errors"
	"log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"tencent-cloud-operator/internal/tencent/common"
	"time"

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

	log.Println("found the securityGroup", *securityGroup.Spec.GroupName)
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
	err = r.securityGroupReconcile(securityGroup)
	if err != nil {
		*securityGroup.Status.Status = "ERROR"
		*securityGroup.Status.LastRetry = time.Now().Format("2006-01-02T15:04:05")
		*securityGroup.Status.RetryCount++
		if cloudError, ok := err.(*tcerrors.TencentCloudSDKError); ok {
			*securityGroup.Status.Code = cloudError.Code
			*securityGroup.Status.Reason = cloudError.Message
		}
		r.Update(context.TODO(), securityGroup)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *SecurityGroupReconciler) securityGroupReconcile(securityGroup *networkv1alpha1.SecurityGroup) error {
	if securityGroup.Status.Status == nil || *securityGroup.Status.Status == "" {
		*securityGroup.Status.Status = "PROCESSING"
		r.Update(context.TODO(), securityGroup)
		return r.createSecurityGroup(securityGroup)
	} else if *securityGroup.Status.Status == "PROCESSING" {
		log.Printf("subnet %s is in PROCESSING status, ignore", *securityGroup.Spec.GroupName)
	} else if *securityGroup.Status.Status == "PENDING" {
		log.Printf("subnet %s is in PENDING status, ignore for now", *securityGroup.Spec.GroupName)
	} else if *securityGroup.Status.Status == "ERROR" {
		lastRetried, _ := time.Parse("2006-01-02T15:04:05", *securityGroup.Status.LastRetry)
		//only retry 10 times, only retry every 1 minute
		if *securityGroup.Status.RetryCount < 10 && time.Now().Sub(lastRetried) > time.Minute {
			if securityGroup.Status.SecurityGroupId == nil || *securityGroup.Status.SecurityGroupId == "" {
				return r.createSecurityGroup(securityGroup)
			} else {
				return r.checkSecurityGroupStatus(securityGroup)
			}
		}
		log.Println("not retrying for subnet:", *securityGroup.Spec.GroupName)
	} else if *securityGroup.Status.Status == "READY" {
		log.Printf("vpc %s is in READY status, check the status from tencnet cloud", *securityGroup.Spec.GroupName)
		return r.checkSecurityGroupStatus(securityGroup)
	}
	return nil
}

func (r *SecurityGroupReconciler) createSecurityGroup(securityGroup *networkv1alpha1.SecurityGroup) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *securityGroup.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewCreateSecurityGroupRequest()
	request.GroupName = securityGroup.Spec.GroupName
	request.GroupDescription = securityGroup.Spec.GroupDescription
	for _, tag := range securityGroup.Spec.Tags {
		log.Printf("tag key:%s, tag value: %s", *tag.Key, *tag.Value)
		request.Tags = append(request.Tags, &tcvpc.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}
	log.Println("request:", request.ToJsonString())
	resp, err := tencentClient.CreateSecurityGroup(request)
	if err != nil {
		log.Println("error create tencent cloud subnet, err:", err)
		return err
	}
	securityGroup.Status.SecurityGroupId = resp.Response.SecurityGroup.SecurityGroupId
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
	request.SecurityGroupIds = append(request.SecurityGroupIds, securityGroup.Status.SecurityGroupId)
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
	*securityGroup.Status.RetryCount = 0
	*securityGroup.Status.LastRetry = ""
	*securityGroup.Status.Code = ""
	*securityGroup.Status.Reason = ""
	return nil
}

func (r *SecurityGroupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.SecurityGroup{}).
		Complete(r)
}
