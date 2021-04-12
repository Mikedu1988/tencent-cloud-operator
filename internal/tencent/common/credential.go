package common

import "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"

var credential = common.NewCredential(
	"AKIDeNGBtPSsmpgFag6GzTivunR0LTO4fryq",
	"6wa4Rnu52lHSRE2eP2WsqkS0GaklZCpO",
)

func GerCredential() *common.Credential {
	return credential
}
