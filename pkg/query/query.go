// Package query holds reusable request-level helpers shared by handlers.
package query

// Page describes a paginated list request.
type Page struct {
	Page    int    `form:"page" json:"page"`
	Size    int    `form:"size" json:"size"`
	Keyword string `form:"keyword" json:"keyword"`
}

// Offset returns the zero-based offset for the current page.
func (p Page) Offset() int {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Size < 1 {
		p.Size = 10
	}
	return (p.Page - 1) * p.Size
}

// Normalize clamps page/size to sane bounds and returns the effective limit.
func (p *Page) Normalize() int {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Size < 1 {
		p.Size = 10
	}
	if p.Size > 200 {
		p.Size = 200
	}
	return p.Size
}
