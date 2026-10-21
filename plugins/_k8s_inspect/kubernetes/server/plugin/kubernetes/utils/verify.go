package utils

import "github.com/flipped-aurora/gin-vue-admin/server/utils"

var (
	ProxyVerify          = utils.Rules{"Path": {utils.NotEmpty()}}
	TerminalVerify       = utils.Rules{"Name": {utils.NotEmpty()}, "PodName": {utils.NotEmpty()}, "Namespace": {utils.NotEmpty()}}
	RoleTypeVerify       = utils.Rules{"RoleType": {utils.NotEmpty()}}
	ApiGroupsVerify      = utils.Rules{"ApiType": {utils.NotEmpty()}}
	RoleVerify           = utils.Rules{"Rules": {utils.NotEmpty()}, "Metadata": {utils.NotEmpty()}}
	PodVerify            = utils.Rules{"ClusterId": {utils.NotEmpty()}}
	DiagnosisVerify      = utils.Rules{"ClusterId": {utils.NotEmpty()}, "PodName": {utils.NotEmpty()}, "ContainerName": {utils.NotEmpty()}, "Namespace": {utils.NotEmpty()}}
	DiagnosisWsVerify    = utils.Rules{"ClusterId": {utils.NotEmpty()}, "PodName": {utils.NotEmpty()}, "ContainerName": {utils.NotEmpty()}, "Namespace": {utils.NotEmpty()}, "XToken": {utils.NotEmpty()}}
	DiagnosisProxyVerify = utils.Rules{"ClusterId": {utils.NotEmpty()}, "PodName": {utils.NotEmpty()}, "ContainerName": {utils.NotEmpty()}, "Namespace": {utils.NotEmpty()}, "Body": {utils.NotEmpty()}}
)
