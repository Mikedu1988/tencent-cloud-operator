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
	"tencent-cloud-operator/internal/tencent/common"
	"time"

	tcerrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	networkv1alpha1 "tencent-cloud-operator/apis/network/v1alpha1"
)

// VpcReconciler reconciles a Vpc object
type VpcReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=network.tencentcloud.kubecooler.com,resources=vpcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.tencentcloud.kubecooler.com,resources=vpcs/status,verbs=get;update;patch

func (r *VpcReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("vpc", req.NamespacedName)
	ctx := context.Background()
	// get the vpc object
	vpc := &networkv1alpha1.Vpc{}
	err := r.Get(ctx, req.NamespacedName, vpc)
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

	log.Println("found the vpc", *vpc.Spec.VpcName)
	if vpc.Status.Status == nil {
		vpc.Status.Status = new(string)
	}
	if vpc.Status.LastRetry == nil {
		vpc.Status.LastRetry = new(string)
	}
	if vpc.Status.Reason == nil {
		vpc.Status.Reason = new(string)
	}
	if vpc.Status.RetryCount == nil {
		vpc.Status.RetryCount = new(int)
	}
	if vpc.Status.Code == nil {
		vpc.Status.Code = new(string)
	}
	err = r.vpcReconcile(vpc)
	if err != nil {
		*vpc.Status.Status = "ERROR"
		*vpc.Status.LastRetry = time.Now().Format("2006-01-02T15:04:05")
		*vpc.Status.RetryCount += 1
		if cloudError, ok := err.(*tcerrors.TencentCloudSDKError); ok {
			*vpc.Status.Code = cloudError.Code
			*vpc.Status.Reason = cloudError.Message
		}
		_ = r.Update(context.TODO(), vpc)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *VpcReconciler) vpcReconcile(vpc *networkv1alpha1.Vpc) error {
	if *vpc.Status.Status == "" || vpc.Status.Status == nil {
		*vpc.Status.Status = "PROCESSING"
		_ = r.Update(context.TODO(), vpc)
		return r.createVpc(vpc)
	} else if *vpc.Status.Status == "PROCESSING" {
		log.Printf("vpc %s is in PROCESSING status, ignore", *vpc.Spec.VpcName)
	} else if *vpc.Status.Status == "ERROR" {
		lastRetried, _ := time.Parse("2006-01-02T15:04:05", *vpc.Status.LastRetry)
		//only retry 10 times, only retry every 1 minute
		if *vpc.Status.RetryCount < 10 && time.Since(lastRetried) > time.Minute {
			if vpc.Status.VpcId == nil || *vpc.Status.VpcId == "" {
				return r.createVpc(vpc)
			} else {
				return r.checkVpcStatus(vpc)
			}
		}
		log.Println("not retrying for vpc:", *vpc.Spec.VpcName)
	} else if *vpc.Status.Status == "READY" {
		log.Printf("vpc %s is in READY status, check the status from tencnet cloud", *vpc.Spec.VpcName)
		return r.checkVpcStatus(vpc)
	}
	return nil
}

func (r *VpcReconciler) createVpc(vpc *networkv1alpha1.Vpc) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *vpc.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewCreateVpcRequest()
	request.VpcName = vpc.Spec.VpcName
	request.CidrBlock = vpc.Spec.CidrBlock
	request.EnableMulticast = vpc.Spec.EnableMulticast
	request.DnsServers = vpc.Spec.DnsServers
	request.DomainName = vpc.Spec.DomainName
	for _, tag := range vpc.Spec.Tags {
		log.Printf("tag key:%s, tag value: %s", *tag.Key, *tag.Value)
		request.Tags = append(request.Tags, &tcvpc.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}
	log.Println("request:", request.ToJsonString())
	resp, err := tencentClient.CreateVpc(request)
	if err != nil {
		log.Println("error create tencent cloud subnet, err:", err)
		return err
	}
	vpc.Status.VpcId = resp.Response.Vpc.VpcId
	*vpc.Status.Status = "READY"
	err = r.Update(context.TODO(), vpc)
	if err != nil {
		log.Println("error update vpc status")
		return err
	}
	return nil
}

func (r *VpcReconciler) checkVpcStatus(vpc *networkv1alpha1.Vpc) error {
	tencentClient, _ := tcvpc.NewClient(common.GerCredential(), *vpc.Spec.Region, profile.NewClientProfile())
	request := tcvpc.NewDescribeVpcsRequest()
	request.VpcIds = append(request.VpcIds, vpc.Status.VpcId)
	resp, err := tencentClient.DescribeVpcs(request)
	if err != nil {
		log.Println("failed to get vpc from tencent cloud, requeue")
		*vpc.Status.RetryCount++
		*vpc.Status.Reason = err.Error()
		if *vpc.Status.RetryCount > 10 {
			return err
		}
		return nil
	}
	if *resp.Response.TotalCount == 0 {
		log.Println("Resource is deleted from cloud, update status")
		*vpc.Status.Status = "DELETED_IN_CLOUD"
		err := r.Update(context.Background(), vpc)
		if err != nil {
			log.Println("error update DELETED_IN_CLOUD")
		}
	}
	*vpc.Status.RetryCount = 0
	*vpc.Status.LastRetry = ""
	*vpc.Status.Code = ""
	*vpc.Status.Reason = ""
	return nil
}

func ignoreUpdatePredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			log.Println("update event")
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.MetaOld.GetGeneration() != e.MetaNew.GetGeneration()
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			log.Println("delete event")
			// Evaluates to false if the object has been confirmed deleted.
			return !e.DeleteStateUnknown
		},
	}
}

func (r *VpcReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkv1alpha1.Vpc{}).
		WithEventFilter(ignoreUpdatePredicate()).
		Complete(r)
}
