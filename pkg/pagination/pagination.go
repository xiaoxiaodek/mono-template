package pagination

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type Page struct {
	Page     int
	PageSize int
	Offset   int
}

func Normalize(page, pageSize int) Page {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return Page{
		Page:     page,
		PageSize: pageSize,
		Offset:   (page - 1) * pageSize,
	}
}
