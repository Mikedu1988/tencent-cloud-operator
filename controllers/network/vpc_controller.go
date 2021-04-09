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
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	tcvpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	"k8s.io/apimachinery/pkg/api/errors"
	"log"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

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

	err = r.vpcReconcile(vpc)
	if err != nil {
		patch := client.MergeFrom(vpc.DeepCopy())
		vpc.Status.Status = new(string)
		*vpc.Status.Status = "ERROR"
		r.Patch(context.TODO(), vpc, patch)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

func (r *VpcReconciler) vpcReconcile(vpc *networkv1alpha1.Vpc) error {
	credential := common.NewCredential(
		"AKIDeNGBtPSsmpgFag6GzTivunR0LTO4fryq",
		"6wa4Rnu52lHSRE2eP2WsqkS0GaklZCpO",
	)
	tencent_client, _ := tcvpc.NewClient(credential, *vpc.Spec.Region, profile.NewClientProfile())
	if *vpc.Status.Status == "" || vpc.Status.Status == nil {
		*vpc.Status.Status = "PROCESSING"
		r.Update(context.TODO(), vpc)
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
		resp, err := tencent_client.CreateVpc(request)
		if err != nil {
			log.Println("error create tencent cloud vpc, err:", err)
			return err
		}
		vpc.Status.VpcId = resp.Response.Vpc.VpcId
		*vpc.Status.Status = "CREATED"
		err = r.Update(context.TODO(), vpc)
		if err != nil {
			log.Println("error update vpc status")
			return err
		}
	} else if *vpc.Status.Status == "PROCESSING" {
		log.Printf("vpc %s is in PROCESSING status, ignore", *vpc.Spec.VpcName)
	} else if *vpc.Status.Status == "CREATED" {
		log.Printf("vpc %s is in CREATED status", *vpc.Spec.VpcName)
		request := tcvpc.NewDescribeVpcsRequest()
		request.VpcIds = append(request.VpcIds, vpc.Status.VpcId)
		resp, err := tencent_client.DescribeVpcs(request)
		if err != nil {
			log.Println("failed to get vpc from tencent cloud, requeue")
			return err
		}
		if *resp.Response.TotalCount == 0 {
			log.Println("Resource is deleted from cloud, update status")
			//patch := client.MergeFrom(vpc)
			*vpc.Status.Status = "DELETED_IN_CLOUD"
			err := r.Update(context.Background(), vpc)
			if err != nil {
				log.Println("error update DELETED_IN_CLOUD")
			}
		}
	}
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
