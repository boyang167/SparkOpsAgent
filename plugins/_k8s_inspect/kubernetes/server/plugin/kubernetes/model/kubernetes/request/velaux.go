package request

type VelauxRequest struct {
	Query    string `form:"query" binding:"required"`
	Velaql   string `form:"velaql" binding:"required"`
	Page     int    `form:"page" binding:"required"`
	PageSize int    `form:"pageSize" binding:"required"`
}
