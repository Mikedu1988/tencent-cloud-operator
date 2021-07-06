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
	tctke "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20180525"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"log"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	networkv1alpha1 "tencent-cloud-operator/apis/network/v1alpha1"
	"tencent-cloud-operator/internal/common"
	"tencent-cloud-operator/internal/utils"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	computev1alpha1 "tencent-cloud-operator/apis/compute/v1alpha1"
)

// ManagedClusterReconciler reconciles a ManagedCluster object
type ManagedClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=compute.tencentcloud.kubecooler.com,resources=managedclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compute.tencentcloud.kubecooler.com,resources=managedclusters/status,verbs=get;update;patch

func (r *ManagedClusterReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("managedcluster", req.NamespacedName)

	// your logic here
	managedCluster := &computev1alpha1.ManagedCluster{}
	err := r.Get(ctx, req.NamespacedName, managedCluster)
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
	if managedCluster.Status.ResourceStatus == nil {
		managedCluster.Status.ResourceStatus = new(common.ResourceStatus)
		managedCluster.Status.ResourceStatus.Status = new(string)
		managedCluster.Status.ResourceStatus.Reason = new(string)
		managedCluster.Status.ResourceStatus.RetryCount = new(int)
		managedCluster.Status.ResourceStatus.Code = new(string)
		managedCluster.Status.ResourceStatus.LastRetry = new(string)
	}
	if managedCluster.Status.Cluster == nil {
		managedCluster.Status.Cluster = new(tctke.Cluster)
	}
	log.Println("found the cluster", *managedCluster.Spec.Cluster.ClusterName)
	err = r.managedClusterReconcile(managedCluster)
	if err != nil {
		*managedCluster.Status.ResourceStatus.Status = "ERROR"
		*managedCluster.Status.ResourceStatus.LastRetry = time.Now().Format("2006-01-02T15:04:05")
		*managedCluster.Status.ResourceStatus.RetryCount++
		*managedCluster.Status.ResourceStatus.Reason = err.Error()
		if cloudError, ok := err.(*tcerrors.TencentCloudSDKError); ok {
			*managedCluster.Status.ResourceStatus.Code = cloudError.Code
			*managedCluster.Status.ResourceStatus.Reason = cloudError.Message
		}
		err = r.Update(context.Background(), managedCluster)
		if err != nil {
			log.Println("error update retry status")
		}
		return ctrl.Result{RequeueAfter: common.RequeueInterval}, err
	}
	return ctrl.Result{RequeueAfter: common.RequeueInterval}, nil
}

func (r *ManagedClusterReconciler) managedClusterReconcile(managedCluster *computev1alpha1.ManagedCluster) error {
	// always check for finalizers
	deleted := !managedCluster.GetDeletionTimestamp().IsZero()
	pendingFinalizers := managedCluster.GetFinalizers()
	finalizerExists := len(pendingFinalizers) > 0
	if !finalizerExists && !deleted && !utils.Contains(pendingFinalizers, common.Finalizer) {
		log.Println("Adding finalized &s to resource", common.Finalizer)
		finalizers := append(pendingFinalizers, common.Finalizer)
		managedCluster.SetFinalizers(finalizers)
		err := r.Update(context.TODO(), managedCluster)
		if err != nil {
			return err
		}
	}

	//if the resource is not created yet, try to create. processing status will only happen once
	if managedCluster.Status.ResourceStatus.Status == nil || *managedCluster.Status.ResourceStatus.Status == "" {
		*managedCluster.Status.ResourceStatus.Status = "PROCESSING"
		return r.Update(context.TODO(), managedCluster)
	}
	if strings.EqualFold(*managedCluster.Status.ResourceStatus.Status, "PROCESSING") {
		return r.createManagedCluster(managedCluster)
	}
	log.Printf("managedCluster %s is in %s status", *managedCluster.Spec.Cluster.ClusterName, *managedCluster.Status.ResourceStatus.Status)

	tencentManagedCluster, err := r.getManagedCluster(managedCluster)
	if err != nil {
		//only report error if resource is not marked as deleted
		log.Printf("error retrive managedCluster %s status from tencent cloud, just requeue for retry", *managedCluster.Spec.Cluster.ClusterName)
		return err
	}
	if deleted {
		if !strings.EqualFold(*managedCluster.Status.ResourceStatus.Status, "DELETING") {
			*managedCluster.Status.ResourceStatus.Status = "DELETING"
			return r.Update(context.TODO(), managedCluster)
		}
		// resource is marked to be deleted, cloud resource still exists
		if tencentManagedCluster != nil {
			err := r.deleteManagedCluster(managedCluster)
			if err != nil {
				r.Log.Error(err, "error delete manager cluster", "namespace:", managedCluster.Namespace, "name:", *managedCluster.Spec.Cluster.ClusterName)
				//error delete the resource from cloud, don't remove finalizer yet
				return nil
			}
		}
		// resource deleted from cloud, remove finalizer
		finalizers := make([]string, 0)
		pendingFinalizers = managedCluster.GetFinalizers()
		for _, pendingFinalizer := range pendingFinalizers {
			if pendingFinalizer != common.Finalizer {
				finalizers = append(finalizers, pendingFinalizer)
			}
		}
		managedCluster.SetFinalizers(finalizers)
		return r.Update(context.TODO(), managedCluster)
	}
	if strings.EqualFold(*managedCluster.Status.ResourceStatus.Status, "PENDING") || strings.EqualFold(*managedCluster.Status.ResourceStatus.Status, "ERROR") {
		lastRetried, _ := time.Parse("2006-01-02T15:04:05", *managedCluster.Status.ResourceStatus.LastRetry)
		//only retry 10 times, only retry every 1 minute
		if *managedCluster.Status.ResourceStatus.RetryCount < 10 && time.Since(lastRetried) > time.Minute {
			if managedCluster.Status.Cluster.ClusterId == nil || *managedCluster.Status.Cluster.ClusterId == "" {
				log.Printf("managedCluster %s is in %s status, retry create", *managedCluster.Spec.Cluster.ClusterName, *managedCluster.Status.ResourceStatus.Status)
				return r.createManagedCluster(managedCluster)
			}
		}
	}
	if !reflect.DeepEqual(managedCluster.Status.Cluster, tencentManagedCluster) || *managedCluster.Status.ResourceStatus.Status != "RUNNING" {
		log.Println("managedCluster not deep equal")
		managedCluster.Status.Cluster = tencentManagedCluster
		*managedCluster.Status.ResourceStatus.Status = strings.ToUpper(*tencentManagedCluster.ClusterStatus)
		*managedCluster.Status.ResourceStatus.RetryCount = 0
		*managedCluster.Status.ResourceStatus.LastRetry = ""
		*managedCluster.Status.ResourceStatus.Code = ""
		*managedCluster.Status.ResourceStatus.Reason = ""
		return r.Update(context.TODO(), managedCluster)
	}
	return nil
}

func (r *ManagedClusterReconciler) createManagedCluster(managedCluster *computev1alpha1.ManagedCluster) error {
	tencentClient, _ := tctke.NewClient(common.GerCredential(), *managedCluster.Spec.Region, profile.NewClientProfile())
	k8sRequest := types.NamespacedName{
		Name:      *managedCluster.Spec.VpcRef.Name,
		Namespace: managedCluster.Namespace,
	}
	vpc := &networkv1alpha1.Vpc{}
	err := r.Get(context.TODO(), k8sRequest, vpc)
	if err != nil {
		r.Log.Info("can't find vpc ref", "namespace", managedCluster.Namespace, "vpcname", managedCluster.Spec.VpcRef.Name)
		return err
	}
	if *vpc.Status.ResourceStatus.Status != "READY" {
		r.Log.Info("referenced vpc is not in READY status", "namespace", managedCluster.Namespace, "vpcname", managedCluster.Spec.VpcRef.Name, "vpc status", vpc.Status.ResourceStatus.Status)
		return &common.ReferencedResourceNotReady{
			Message: fmt.Sprintf("referenced vpc is not in READY status. namespace %s, vpcname: %s, vpc status: %s", managedCluster.Namespace, *managedCluster.Spec.VpcRef.Name, *vpc.Status.ResourceStatus.Status),
		}
	}
	request := tctke.NewCreateClusterRequest()
	request.ClusterBasicSettings = new(tctke.ClusterBasicSettings)
	request.ClusterCIDRSettings = new(tctke.ClusterCIDRSettings)
	request.ClusterBasicSettings.ClusterName = managedCluster.Spec.Cluster.ClusterName
	request.ClusterBasicSettings.ClusterDescription = managedCluster.Spec.Cluster.ClusterDescription
	request.ClusterBasicSettings.ClusterOs = managedCluster.Spec.Cluster.ClusterOs
	request.ClusterBasicSettings.OsCustomizeType = managedCluster.Spec.Cluster.OsCustomizeType
	request.ClusterCIDRSettings.ClusterCIDR = managedCluster.Spec.Cluster.ClusterNetworkSettings.ClusterCIDR
	request.ClusterBasicSettings.ClusterVersion = managedCluster.Spec.Cluster.ClusterVersion
	request.ClusterType = managedCluster.Spec.Cluster.ClusterType
	request.ClusterBasicSettings.VpcId = vpc.Status.VpcId
	resp, err := tencentClient.CreateCluster(request)
	if err != nil {
		log.Println("error create tencent cloud Managed cluster, err:", err)
		return err
	}
	managedCluster.Status.Cluster.ClusterId = resp.Response.ClusterId
	*managedCluster.Status.ResourceStatus.Status = "CREATING"
	err = r.Update(context.TODO(), managedCluster)
	if err != nil {
		log.Println("error update managedCluster status")
		return err
	}
	return nil
}

func (r *ManagedClusterReconciler) getManagedCluster(managedCluster *computev1alpha1.ManagedCluster) (*tctke.Cluster, error) {
	//cloud resource is not created, return nil
	if managedCluster.Status.Cluster.ClusterId == nil || *managedCluster.Status.Cluster.ClusterId == "" {
		return nil, nil
	}
	tencentClient, _ := tctke.NewClient(common.GerCredential(), *managedCluster.Spec.Region, profile.NewClientProfile())
	request := tctke.NewDescribeClustersRequest()
	request.ClusterIds = append(request.ClusterIds, managedCluster.Status.Cluster.ClusterId)
	resp, err := tencentClient.DescribeClusters(request)
	if err != nil {
		return nil, err
	}
	if *resp.Response.TotalCount == 0 {
		log.Println("Resource is deleted from cloud, update status")
		//the resource is not marked as deleted, but deleted in cloud
		return nil, nil
	}
	return resp.Response.Clusters[0], nil
}

func (r *ManagedClusterReconciler) deleteManagedCluster(managedCluster *computev1alpha1.ManagedCluster) error {
	tencentClient, _ := tctke.NewClient(common.GerCredential(), *managedCluster.Spec.Region, profile.NewClientProfile())
	request := tctke.NewDeleteClusterRequest()
	request.ClusterId = managedCluster.Status.Cluster.ClusterId
	request.InstanceDeleteMode = new(string)
	*request.InstanceDeleteMode = "terminate"
	_, err := tencentClient.DeleteCluster(request)
	if err != nil {
		return err
	}
	return nil
}

func (r *ManagedClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1alpha1.ManagedCluster{}).
		Complete(r)
}
