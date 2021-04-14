package common

import (
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"os"
)

var credential = common.NewCredential(
	os.Getenv("TENCENT_CLOUD_ACCESS_KEY"),
	os.Getenv("TENCENT_CLOUD_ACCESS_SECRET"),
)

func GerCredential() *common.Credential {
	return credential
}
