package response

// Pagination is camelCase page meta for server-side list endpoints.
type Pagination struct {
	Page       int `json:"page"`
	PerPage    int `json:"perPage"`
	TotalCount int `json:"totalCount"`
	TotalPages int `json:"totalPages"`
}

// NormalizePage applies defaults: page≤0→1, perPage≤0→10, perPage max 100.
func NormalizePage(page, perPage int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}

// NewPagination builds meta from page size and total row count.
func NewPagination(page, perPage, totalCount int) Pagination {
	totalPages := 0
	if perPage > 0 && totalCount > 0 {
		totalPages = (totalCount + perPage - 1) / perPage
	}
	return Pagination{
		Page: page, PerPage: perPage, TotalCount: totalCount, TotalPages: totalPages,
	}
}

// Offset returns SQL/Keycloak-style offset for a 1-based page.
func Offset(page, perPage int) int {
	page, perPage = NormalizePage(page, perPage)
	return (page - 1) * perPage
}
