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
	"fmt"
	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

// SubnetReconciler reconciles a Subnet object
type SubnetReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=network.tencentcloud.kubecooler.com,resources=subnets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.tencentcloud.kubecooler.com,resources=subnets/status,verbs=get;update;patch

func (r *SubnetReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("subnet", req.NamespacedName)
	ctx := context.TODO()
	// your logic here
	subnet := &networkv1alpha1.Subnet{}
	err := r.Get(ctx, req.NamespacedName, subnet)
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

	log.Println("found the subnet", *subnet.Spec.SubnetName)
	if subnet.Status.Status == nil {
		subnet.Status.Status = new(string)
	}
	if subnet.Status.Vpc == nil {
		subnet.Status.Vpc = new(networkv1alpha1.VpcStatus)
	}
	if subnet.Status.LastRetry == nil {
		subnet.Status.LastRetry = new(string)
	}
	if subnet.Status.Reason == nil {
		subnet.Status.Reason = new(string)
	}
	if subnet.Status.RetryCount == nil {
		subnet.Status.RetryCount = new(int)
	}
	if subnet.Status.Code == nil {
		subnet.Status.Code = new(string)
	}
	err = r.subnetReconcile(subnet)
	if err != nil {
		if _, ok := err.(*ReferencedResourceNotReady); ok {
			r.Log.Info("ref vpc not ready, requeue after 30 seconds")
			*subnet.Status.Status = "PENDING"
			*subnet.Status.Reason = "ref vpc not ready"
			r.Update(context.TODO(), subnet)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		*subnet.Status.Status = "ERROR"
		*subnet.Status.LastRetry = time.Now().Format("2006-01-02T15:04:05")
		*subnet.Status.RetryCount += 1
		if cloudError, ok := err.(*tcerrors.TencentCloudSDKError); ok {
			*subnet.Status.Code = cloudError.Code
			*subnet.Status.Reason = cloudError.Message
		}
		r.Update(context.TODO(), subnet)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	return ctrl.Result{}, nil
}

func (r *SubnetReconciler) subnetReconcile(subnet *networkv1alpha1.Subnet) error {
	if subnet.Status.Status == nil || *subnet.Status.Status == "" {
		*subnet.Status.Status = "PROCESSING"
		r.Update(context.TODO(), subnet)
		return r.createSubnet(subnet)
	} else if *subnet.Status.Status == "PROCESSING" {
		log.Printf("subnet %s is in PROCESSING status, ignore", *subnet.Spec.SubnetName)
	} else if *subnet.Status.Status == "PENDING" {
		log.Printf("subnet %s is in PENDING status, ignore for now", *subnet.Spec.SubnetName)
	} else if *subnet.Status.Status == "ERROR" {
		lastRetried, _ := time.Parse("2006-01-02T15:04:05", *subnet.Status.LastRetry)
		//only retry 10 times, only retry every 1 minute
		if *subnet.Status.RetryCount < 10 && time.Now().Sub(lastRetried) > time.Minute {
			if subnet.Status.SubnetId == nil || *subnet.Status.SubnetId == "" {
				return r.createSubnet(subnet)
			} else {
				return r.checkSubnetStatus(subnet)
			}
		}
		log.Println("not retrying for subnet:", *subnet.Spec.SubnetName)
	} else if *subnet.Status.Status == "READY" {
		log.Printf("vpc %s is in READY status, check the status from tencnet cloud", *subnet.Spec.SubnetName)
		return r.checkSubnetStatus(subnet)
	}
	return nil
}

func (r *SubnetReconciler) createSubnet(subnet *networkv1alpha1.Subnet) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *subnet.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewCreateSubnetRequest()
	request.SubnetName = subnet.Spec.SubnetName
	request.CidrBlock = subnet.Spec.CidrBlock
	request.Zone = subnet.Spec.Zone
	for _, tag := range subnet.Spec.Tags {
		log.Printf("tag key:%s, tag value: %s", *tag.Key, *tag.Value)
		request.Tags = append(request.Tags, &tcvpc.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}
	//if vpc id is not empty, use vpc id
	if subnet.Spec.VpcId == nil || *subnet.Spec.VpcId == "" {
		k8sRequest := types.NamespacedName{
			Name:      *subnet.Spec.VpcRef.Name,
			Namespace: subnet.Namespace,
		}
		vpc := &networkv1alpha1.Vpc{}
		err := r.Get(context.TODO(), k8sRequest, vpc)
		if err != nil {
			r.Log.Info("can't find vpc ref", "namespace", subnet.Namespace, "vpcname", subnet.Spec.VpcRef.Name)
			return err
		}
		if *vpc.Status.Status != "READY" {
			r.Log.Info("referenced vpc is not in READY status", "namespace", subnet.Namespace, "vpcname", subnet.Spec.VpcRef.Name, "vpc status", vpc.Status.Status)
			return &ReferencedResourceNotReady{
				Message: fmt.Sprintf("referenced vpc is not in READY status. namespace %s, vpcname: %s, vpc status: %s", subnet.Namespace, *subnet.Spec.VpcRef.Name, *vpc.Status.Status),
			}
		}
		request.VpcId = vpc.Status.VpcId
	} else {
		request.VpcId = subnet.Spec.VpcId
	}
	log.Println("request:", request.ToJsonString())
	resp, err := tencentClient.CreateSubnet(request)
	if err != nil {
		log.Println("error create tencent cloud subnet, err:", err)
		return err
	}
	subnet.Status.SubnetId = resp.Response.Subnet.SubnetId
	*subnet.Status.Status = "READY"
	subnet.Status.Vpc.VpcId = resp.Response.Subnet.VpcId
	err = r.Update(context.TODO(), subnet)
	if err != nil {
		log.Println("error update vpc status")
		return err
	}
	return nil
}

func (r *SubnetReconciler) checkSubnetStatus(subnet *networkv1alpha1.Subnet) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *subnet.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewDescribeSubnetsRequest()
	request.SubnetIds = append(request.SubnetIds, subnet.Status.SubnetId)
	resp, err := tencentClient.DescribeSubnets(request)
	if err != nil {
		log.Println("failed to get subnet from tencent cloud, requeue")
		*subnet.Status.RetryCount++
		*subnet.Status.Reason = err.Error()
		if *subnet.Status.RetryCount > 10 {
			return err
		}
		return nil
	}
	if *resp.Response.TotalCount == 0 {
		log.Println("Resource is deleted from cloud, update status")
		*subnet.Status.Status = "DELETED_IN_CLOUD"
		err := r.Update(context.Background(), subnet)
		if err != nil {
			log.Println("error update DELETED_IN_CLOUD")
		}
	}
	*subnet.Status.RetryCount = 0
	*subnet.Status.LastRetry = ""
	*subnet.Status.Code = ""
	*subnet.Status.Reason = ""
	return nil
}

func (r *SubnetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.Subnet{}).
		WithEventFilter(ignoreUpdatePredicate()).
		Complete(r)
}
