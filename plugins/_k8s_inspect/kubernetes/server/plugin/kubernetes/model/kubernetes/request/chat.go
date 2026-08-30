package request

type ChatReq struct {
	ClusterId int    `json:"cluster_id"`
	Question  string `json:"question"`
}
