package repository

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"scm/internal/model"
)

// CustomerRepo owns base_customer.
type CustomerRepo struct{ *genericRepo[model.Customer] }

func NewCustomerRepo(db *gormDB) *CustomerRepo {
	return &CustomerRepo{genericRepo: newGenericRepo[model.Customer](db)}
}

func (r *CustomerRepo) List(f ListFilter, out *[]model.Customer, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("customer_code LIKE ? OR name LIKE ?", like, like)
		}
		return q.Order("id DESC")
	}
	return r.list(f, apply, out, total)
}

// SaleOrderRepo owns sale_order + sale_order_detail.
type SaleOrderRepo struct {
	*genericRepo[model.SaleOrder]
	db *gormDB
}

func NewSaleOrderRepo(db *gormDB) *SaleOrderRepo {
	return &SaleOrderRepo{
		genericRepo: newGenericRepo[model.SaleOrder](db),
		db:          db,
	}
}

// GetWithDetails loads an SO with its details preloaded.
func (r *SaleOrderRepo) GetWithDetails(id uint) (*model.SaleOrder, error) {
	var so model.SaleOrder
	if err := r.db.DB.Preload("Details").First(&so, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &so, nil
}

// CreateWithDetails inserts an SO and its details in one transaction. The
// header is created with associations omitted so the explicit detail loop
// stays the single writer.
func (r *SaleOrderRepo) CreateWithDetails(so *model.SaleOrder) error {
	return r.db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(so).Error; err != nil {
			return err
		}
		for i := range so.Details {
			so.Details[i].SOID = so.ID
			if err := tx.Create(&so.Details[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SaleOrderRepo) List(f ListFilter, out *[]model.SaleOrder, total *int64) error {
	apply := func(q *gorm.DB) *gorm.DB {
		if f.Keyword != "" {
			like := "%" + f.Keyword + "%"
			q = q.Where("so_number LIKE ?", like)
		}
		return q.Order("id DESC")
	}
	return r.list(f, apply, out, total)
}
