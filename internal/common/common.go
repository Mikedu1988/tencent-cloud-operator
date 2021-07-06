package common

import (
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"os"
	"time"
)

const (
	Finalizer = "finalized.tencentcloud.kubecooler.com"
)

var RequeueInterval = 60 * time.Second

var credential = common.NewCredential(
	os.Getenv("TENCENTCLOUD_SECRET_ID"),
	os.Getenv("TENCENTCLOUD_SECRET_KEY"),
)

func GerCredential() *common.Credential {
	return credential
}

type ResourceStatus struct {
	Status     *string `json:"status,omitempty"`
	Reason     *string `json:"reason,omitempty"`
	Code       *string `json:"code,omitempty"`
	LastRetry  *string `json:"lastRetry,omitempty"`
	RetryCount *int    `json:"retryCount,omitempty"`
}
