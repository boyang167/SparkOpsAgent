package kernel

type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

func NewPage[T any](items []T, total int64, page, pageSize int) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}
