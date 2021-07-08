package common

import (
	"os"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
)

const (
	//Finalizer the finalizer add to all resources managed by this operator
	Finalizer = "finalized.tencentcloud.kubecooler.com"
	//RequeueInterval reconcile interval
	RequeueInterval = 60 * time.Second
)

var credential = common.NewCredential(
	os.Getenv("TENCENTCLOUD_SECRET_ID"),
	os.Getenv("TENCENTCLOUD_SECRET_KEY"),
)

//GerCredential get tencent cloud ak,sk.
func GerCredential() *common.Credential {
	return credential
}

//ResourceStatus all resources should update ResourceStatus during transition
type ResourceStatus struct {
	Status     *string `json:"status,omitempty"`
	Reason     *string `json:"reason,omitempty"`
	Code       *string `json:"code,omitempty"`
	LastRetry  *string `json:"lastRetry,omitempty"`
	RetryCount *int    `json:"retryCount,omitempty"`
}
