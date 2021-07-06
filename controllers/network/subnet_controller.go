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
	"log"
	"strings"
	"tencent-cloud-operator/internal/common"
	"tencent-cloud-operator/internal/utils"
	"time"

	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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

	log.Println("found the subnet", *subnet.Spec.Subnet.SubnetName)
	if subnet.Status.ResourceStatus == nil {
		subnet.Status.ResourceStatus = new(common.ResourceStatus)
		subnet.Status.ResourceStatus.Status = new(string)
		subnet.Status.ResourceStatus.Reason = new(string)
		subnet.Status.ResourceStatus.RetryCount = new(int)
		subnet.Status.ResourceStatus.Code = new(string)
		subnet.Status.ResourceStatus.LastRetry = new(string)
	}
	if subnet.Status.Subnet == nil {
		subnet.Status.Subnet = new(tcvpc.Subnet)
	}
	if subnet.Status.Vpc == nil {
		subnet.Status.Vpc = new(tcvpc.Vpc)
	}
	err = r.subnetReconcile(subnet)
	if err != nil {
		if _, ok := err.(*common.ReferencedResourceNotReady); ok {
			r.Log.Info("ref vpc not ready, requeue after 30 seconds")
			*subnet.Status.ResourceStatus.Status = "PENDING"
			*subnet.Status.ResourceStatus.Reason = "ref vpc not ready"
			_ = r.Update(context.TODO(), subnet)
			return ctrl.Result{RequeueAfter: common.RequeueInterval}, nil
		}
		*subnet.Status.ResourceStatus.Status = "ERROR"
		*subnet.Status.ResourceStatus.LastRetry = time.Now().UTC().Format("2006-01-02T15:04:05")
		*subnet.Status.ResourceStatus.RetryCount += 1
		if cloudError, ok := err.(*tcerrors.TencentCloudSDKError); ok {
			*subnet.Status.ResourceStatus.Code = cloudError.Code
			*subnet.Status.ResourceStatus.Reason = cloudError.Message
		}
		_ = r.Update(context.TODO(), subnet)
		return ctrl.Result{RequeueAfter: common.RequeueInterval}, err
	}
	return ctrl.Result{RequeueAfter: common.RequeueInterval}, nil
}

func (r *SubnetReconciler) subnetReconcile(subnet *networkv1alpha1.Subnet) error {
	// always check for finalizers
	deleted := !subnet.GetDeletionTimestamp().IsZero()
	pendingFinalizers := subnet.GetFinalizers()
	finalizerExists := len(pendingFinalizers) > 0
	if !finalizerExists && !deleted && !utils.Contains(pendingFinalizers, common.Finalizer) {
		log.Println("Adding finalized &s to resource", common.Finalizer)
		finalizers := append(pendingFinalizers, common.Finalizer)
		subnet.SetFinalizers(finalizers)
		err := r.Update(context.TODO(), subnet)
		if err != nil {
			return err
		}
	}
	if subnet.Status.ResourceStatus.Status == nil || *subnet.Status.ResourceStatus.Status == "" {
		*subnet.Status.ResourceStatus.Status = "PROCESSING"
		return r.Update(context.TODO(), subnet)

	}
	if *subnet.Status.ResourceStatus.Status == "PROCESSING" {
		return r.createSubnet(subnet)
	}
	log.Printf("subnet %s is in %s status", *subnet.Spec.Subnet.SubnetName, *subnet.Status.ResourceStatus.Status)
	tencentSubnet, err := r.getSubnet(subnet)
	// err get resource from cloud, and resource not marked as deleted, something wrong
	if err != nil {
		log.Printf("error retrive subnet %s status from tencent cloud, just requeue for retry", *subnet.Spec.Subnet.SubnetName)
		return err
	}
	if deleted {
		if !strings.EqualFold(*subnet.Status.ResourceStatus.Status, "DELETING") && !strings.EqualFold(*subnet.Status.ResourceStatus.Status, "ERROR") {
			*subnet.Status.ResourceStatus.Status = "DELETING"
			return r.Update(context.TODO(), subnet)
		}
		// resource is marked to be deleted, cloud resource still exists
		if tencentSubnet != nil {
			err := r.deleteSubnet(subnet)
			if err != nil {
				r.Log.Info("error delete subnet", "error delete subnet", "namespace:", subnet.Namespace, "name:", *subnet.Spec.Subnet.SubnetName)
				//error delete the resource from cloud, don't remove finalizer yet
				return nil
			}
		}
		// resource deleted from cloud, remove finalizer
		finalizers := make([]string, 0)
		pendingFinalizers = subnet.GetFinalizers()
		for _, pendingFinalizer := range pendingFinalizers {
			if pendingFinalizer != common.Finalizer {
				finalizers = append(finalizers, pendingFinalizer)
			}
		}
		subnet.SetFinalizers(finalizers)
		return r.Update(context.TODO(), subnet)
	}
	//resource not marked as deleted, and in PENDING, ERROR status
	if strings.EqualFold(*subnet.Status.ResourceStatus.Status, "PENDING") || strings.EqualFold(*subnet.Status.ResourceStatus.Status, "ERROR") {
		lastRetried, _ := time.Parse("2006-01-02T15:04:05", *subnet.Status.ResourceStatus.LastRetry)
		log.Println("time since:",time.Since(lastRetried))
		//only retry 10 times, only retry every 1 minute
		if *subnet.Status.ResourceStatus.RetryCount < 10 && time.Since(lastRetried) > time.Minute {
			log.Printf("subnet %s is in  status, retry", *subnet.Spec.Subnet.SubnetName)
			if subnet.Status.Subnet.SubnetId == nil || *subnet.Status.Subnet.SubnetId == "" {
				return r.createSubnet(subnet)
			}
		}
	}
	//resource deleted in cloud, update the status
	if tencentSubnet == nil {
		if strings.EqualFold(*subnet.Status.ResourceStatus.Status, "READY") {
			*subnet.Status.ResourceStatus.RetryCount = 0
			*subnet.Status.ResourceStatus.LastRetry = ""
			*subnet.Status.ResourceStatus.Code = ""
			*subnet.Status.ResourceStatus.Reason = "Subnet deleted in cloud"
			*subnet.Status.ResourceStatus.Status = "ERROR"
			return r.Update(context.TODO(), subnet)
		}
		return nil
	}
	//get resource from tencent cloud, and resource not marked as deleted, update status
	if !strings.EqualFold(*subnet.Status.ResourceStatus.Code, "") || !strings.EqualFold(*subnet.Status.ResourceStatus.Reason, "") || !strings.EqualFold(*subnet.Status.ResourceStatus.Status, "READY") {
		*subnet.Status.Subnet.SubnetId = *tencentSubnet.SubnetId
		*subnet.Status.ResourceStatus.RetryCount = 0
		*subnet.Status.ResourceStatus.LastRetry = ""
		*subnet.Status.ResourceStatus.Code = ""
		*subnet.Status.ResourceStatus.Reason = ""
		*subnet.Status.ResourceStatus.Status = "READY"
		return r.Update(context.TODO(), subnet)
	}
	return nil
}

func (r *SubnetReconciler) createSubnet(subnet *networkv1alpha1.Subnet) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *subnet.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewCreateSubnetRequest()
	request.SubnetName = subnet.Spec.Subnet.SubnetName
	request.CidrBlock = subnet.Spec.Subnet.CidrBlock
	request.Zone = subnet.Spec.Subnet.Zone
	request.Tags = subnet.Spec.Subnet.TagSet
	if subnet.Spec.Subnet.VpcId == nil || *subnet.Spec.Subnet.VpcId == "" {
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
		if *vpc.Status.ResourceStatus.Status != "READY" {
			r.Log.Info("referenced vpc is not in READY status", "namespace", subnet.Namespace, "vpcname", subnet.Spec.VpcRef.Name, "vpc status", vpc.Status.ResourceStatus.Status)
			return &common.ReferencedResourceNotReady{
				Message: fmt.Sprintf("referenced vpc is not in READY status. namespace %s, vpcname: %s, vpc status: %s", subnet.Namespace, *subnet.Spec.VpcRef.Name, *vpc.Status.ResourceStatus.Status),
			}
		}
		request.VpcId = vpc.Status.VpcId
	} else {
		request.VpcId = subnet.Spec.Subnet.VpcId
	}
	log.Println("request:", request.ToJsonString())
	resp, err := tencentClient.CreateSubnet(request)
	if err != nil {
		log.Println("error create tencent cloud subnet, err:", err)
		return err
	}
	subnet.Status.Subnet.SubnetId = resp.Response.Subnet.SubnetId
	*subnet.Status.ResourceStatus.Status = "READY"
	*subnet.Status.ResourceStatus.RetryCount = 0
	*subnet.Status.ResourceStatus.LastRetry = ""
	*subnet.Status.ResourceStatus.Code = ""
	*subnet.Status.ResourceStatus.Reason = ""
	subnet.Status.Vpc.VpcId = resp.Response.Subnet.VpcId
	err = r.Update(context.TODO(), subnet)
	if err != nil {
		log.Println("error update vpc status")
		return err
	}
	return nil
}

func (r *SubnetReconciler) getSubnet(subnet *networkv1alpha1.Subnet) (*tcvpc.Subnet, error) {
	//cloud resource is not created, return nil
	if subnet.Status.Subnet.SubnetId == nil || *subnet.Status.Subnet.SubnetId == "" {
		return nil, nil
	}
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *subnet.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewDescribeSubnetsRequest()
	request.SubnetIds = append(request.SubnetIds, subnet.Status.Subnet.SubnetId)
	resp, err := tencentClient.DescribeSubnets(request)
	if err != nil {
		log.Println("failed to get subnet from tencent cloud, requeue")
		return nil, err
	}
	if *resp.Response.TotalCount == 0 {
		log.Println("Resource is deleted from cloud, update status")
		return nil, nil
	}
	return resp.Response.SubnetSet[0], nil
}

func (r *SubnetReconciler) deleteSubnet(subnet *networkv1alpha1.Subnet) error {
	if subnet.Status.Subnet.SubnetId == nil || *subnet.Status.Subnet.SubnetId == "" {
		return nil
	}
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *subnet.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewDeleteSubnetRequest()
	request.SubnetId = subnet.Status.Subnet.SubnetId
	_, err := tencentClient.DeleteSubnet(request)
	if err != nil {
		log.Println("failed to delete subnet from tencent cloud, requeue")
		return err
	}
	return nil
}

func (r *SubnetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.Subnet{}).
		WithEventFilter(ignoreUpdatePredicate()).
		Complete(r)
}
