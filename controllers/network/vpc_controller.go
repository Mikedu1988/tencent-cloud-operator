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
	"strings"
	"time"

	"tencent-cloud-operator/internal/common"
	"tencent-cloud-operator/internal/utils"

	log "github.com/sirupsen/logrus"

	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1alpha1 "tencent-cloud-operator/apis/network/v1alpha1"
)

// VpcReconciler reconciles a Vpc object
type VpcReconciler struct {
	client.Client
	Log    *log.Logger
	Scheme *runtime.Scheme
}

//Reconcile start reconcile loop
// +kubebuilder:rbac:groups=network.tencentcloud.kubecooler.com,resources=vpcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.tencentcloud.kubecooler.com,resources=vpcs/status,verbs=get;update;patch
func (r *VpcReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	r.Log.Info("vpc", req.String())
	ctx := context.Background()
	// get the vpc object
	vpc := &networkv1alpha1.Vpc{}
	err := r.Get(ctx, req.NamespacedName, vpc)
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

	r.Log.Info("found the vpc", *vpc.Spec.VpcName)
	if vpc.Status.ResourceStatus == nil {
		vpc.Status.ResourceStatus = new(common.ResourceStatus)
		vpc.Status.ResourceStatus.Status = new(string)
		vpc.Status.ResourceStatus.Reason = new(string)
		vpc.Status.ResourceStatus.RetryCount = new(int)
		vpc.Status.ResourceStatus.Code = new(string)
		vpc.Status.ResourceStatus.LastRetry = new(string)
	}
	if vpc.Status.VpcID == nil {
		vpc.Status.VpcID = new(string)
	}
	err = r.vpcReconcile(vpc)
	if err != nil {
		*vpc.Status.ResourceStatus.Status = "ERROR"
		*vpc.Status.ResourceStatus.LastRetry = time.Now().UTC().Format("2006-01-02T15:04:05")
		*vpc.Status.ResourceStatus.RetryCount++
		if cloudError, ok := err.(*tcerrors.TencentCloudSDKError); ok {
			*vpc.Status.ResourceStatus.Code = cloudError.Code
			*vpc.Status.ResourceStatus.Reason = cloudError.Message
		}
		_ = r.Update(context.TODO(), vpc)
		return ctrl.Result{RequeueAfter: common.RequeueInterval}, err
	}
	return ctrl.Result{RequeueAfter: common.RequeueInterval}, nil
}

func (r *VpcReconciler) vpcReconcile(vpc *networkv1alpha1.Vpc) error {
	// always check for finalizers
	deleted := !vpc.GetDeletionTimestamp().IsZero()
	pendingFinalizers := vpc.GetFinalizers()
	finalizerExists := len(pendingFinalizers) > 0
	if !finalizerExists && !deleted && !utils.Contains(pendingFinalizers, common.Finalizer) {
		r.Log.Info("Adding finalized &s to resource", common.Finalizer)
		finalizers := append(pendingFinalizers, common.Finalizer)
		vpc.SetFinalizers(finalizers)
		return r.Update(context.TODO(), vpc)
	}
	if *vpc.Status.ResourceStatus.Status == "" || vpc.Status.ResourceStatus.Status == nil {
		*vpc.Status.ResourceStatus.Status = "PROCESSING"
		return r.Update(context.TODO(), vpc)
	}
	if *vpc.Status.ResourceStatus.Status == "PROCESSING" {
		return r.createVpc(vpc)
	}
	r.Log.Infof("vpc %s is in %s status", *vpc.Spec.VpcName, *vpc.Status.ResourceStatus.Status)
	tencentVpc, err := r.getVpc(vpc)
	// err get resource from cloud, and resource not marked as deleted, something wrong
	if err != nil {
		r.Log.Infof("error retrieve vpc %s status from tencent cloud, just requeue for retry", *vpc.Spec.VpcName)
		return err
	}
	if deleted {
		// resource marked as deleted, but status not in deleting or error state, update the state to deleting
		if !strings.EqualFold(*vpc.Status.ResourceStatus.Status, "DELETING") && !strings.EqualFold(*vpc.Status.ResourceStatus.Status, "ERROR") {
			*vpc.Status.ResourceStatus.Status = "DELETING"
			return r.Update(context.TODO(), vpc)
		}
		if tencentVpc != nil {
			// resource is marked to be deleted, cloud resource still exists
			lastRetried, _ := time.Parse("2006-01-02T15:04:05", *vpc.Status.ResourceStatus.LastRetry)
			//only retry 10 times, only retry every 1 minute
			if *vpc.Status.ResourceStatus.RetryCount < 10 && time.Since(lastRetried) > time.Minute {
				err = r.deleteVpc(vpc)
				if err != nil {
					r.Log.Info("error delete vpc", "namespace:", vpc.Namespace, "name:", *vpc.Spec.VpcName)
					//error delete the resource from cloud, don't remove finalizer yet
					return err
				}
			}
		}
		// resource deleted from cloud, remove finalizer
		finalizers := make([]string, 0)
		pendingFinalizers = vpc.GetFinalizers()
		for _, pendingFinalizer := range pendingFinalizers {
			if pendingFinalizer != common.Finalizer {
				finalizers = append(finalizers, pendingFinalizer)
			}
		}
		vpc.SetFinalizers(finalizers)
		return r.Update(context.TODO(), vpc)
	}
	//resource not marked as deleted, and get error status, try to create the resource in cloud
	if strings.EqualFold(*vpc.Status.ResourceStatus.Status, "ERROR") {
		lastRetried, _ := time.Parse("2006-01-02T15:04:05", *vpc.Status.ResourceStatus.LastRetry)
		//only retry 10 times, only retry every 1 minute
		if *vpc.Status.ResourceStatus.RetryCount < 10 && time.Since(lastRetried) > time.Minute {
			// resource in error status, retry create
			if vpc.Status.VpcID == nil || *vpc.Status.VpcID == "" {
				r.Log.Info("vpc is in error status, and vpc id is empty, retry create")
				return r.createVpc(vpc)
			}
		}
	}
	//resource deleted in cloud, update the status
	if tencentVpc == nil {
		if strings.EqualFold(*vpc.Status.ResourceStatus.Status, "READY") {
			*vpc.Status.ResourceStatus.RetryCount = 0
			*vpc.Status.ResourceStatus.LastRetry = ""
			*vpc.Status.ResourceStatus.Code = ""
			*vpc.Status.ResourceStatus.Reason = ""
			*vpc.Status.ResourceStatus.Status = "DELETED_IN_CLOUD"
			return r.Update(context.TODO(), vpc)
		}
		return nil
	}
	//get resource from tencent cloud, and resource not marked as deleted, update status
	if !strings.EqualFold(*vpc.Status.ResourceStatus.Code, "") || !strings.EqualFold(*vpc.Status.ResourceStatus.Reason, "") || !strings.EqualFold(*vpc.Status.ResourceStatus.Status, "READY") {
		vpc.Status.VpcID = tencentVpc.VpcId
		*vpc.Status.ResourceStatus.RetryCount = 0
		*vpc.Status.ResourceStatus.LastRetry = ""
		*vpc.Status.ResourceStatus.Code = ""
		*vpc.Status.ResourceStatus.Reason = ""
		*vpc.Status.ResourceStatus.Status = "READY"
		return r.Update(context.TODO(), vpc)
	}
	return nil
}

func (r *VpcReconciler) createVpc(vpc *networkv1alpha1.Vpc) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *vpc.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewCreateVpcRequest()
	request.VpcName = vpc.Spec.VpcName
	request.CidrBlock = vpc.Spec.CidrBlock
	request.EnableMulticast = vpc.Spec.EnableMulticast
	request.DnsServers = vpc.Spec.DNSServers
	request.DomainName = vpc.Spec.DomainName
	for _, tag := range vpc.Spec.Tags {
		request.Tags = append(request.Tags, &tcvpc.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}
	resp, err := tencentClient.CreateVpc(request)
	if err != nil {
		return err
	}
	vpc.Status.VpcID = resp.Response.Vpc.VpcId
	*vpc.Status.ResourceStatus.Status = "READY"
	return r.Update(context.TODO(), vpc)
}

func (r *VpcReconciler) getVpc(vpc *networkv1alpha1.Vpc) (*tcvpc.Vpc, error) {
	if vpc.Status.VpcID == nil || *vpc.Status.VpcID == "" {
		return nil, errors.NewBadRequest("vpc id not found")
	}
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *vpc.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewDescribeVpcsRequest()
	request.VpcIds = append(request.VpcIds, vpc.Status.VpcID)
	resp, err := tencentClient.DescribeVpcs(request)
	if err != nil {
		r.Log.Info("failed to get vpc from tencent cloud, requeue")
		return nil, err
	}
	if *resp.Response.TotalCount == 0 {
		r.Log.Info("Resource is deleted from cloud")
		return nil, nil
	}
	return resp.Response.VpcSet[0], nil
}

func (r *VpcReconciler) deleteVpc(vpc *networkv1alpha1.Vpc) error {
	if vpc.Status.VpcID == nil || *vpc.Status.VpcID == "" {
		return nil
	}
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *vpc.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewDeleteVpcRequest()
	request.VpcId = vpc.Status.VpcID
	_, err := tencentClient.DeleteVpc(request)
	if err != nil {
		r.Log.Info("failed to delete vpc from tencent cloud, requeue")
		return err
	}
	return nil
}

func ignoreUpdatePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.MetaOld.GetGeneration() != e.MetaNew.GetGeneration()
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Evaluates to false if the object has been confirmed deleted.
			return !e.DeleteStateUnknown
		},
	}
}

//SetupWithManager setup controller with manager
func (r *VpcReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.Vpc{}).
		WithEventFilter(ignoreUpdatePredicate()).
		Complete(r)
}
