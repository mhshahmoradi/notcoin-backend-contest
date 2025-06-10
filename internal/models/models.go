package models

import "time"

type Item struct {
	ID        int64     `json:"id"`
	SaleID    int64     `json:"sale_id"`
	Name      string    `json:"name"`
	ImageURL  string    `json:"image_url"`
	IsSold    bool      `json:"is_sold"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Sale struct {
	ID         int64     `json:"id"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	TotalItems int       `json:"total_items"`
	SoldItems  int       `json:"sold_items"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CheckoutAttempt struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ItemID    int64     `json:"item_id"`
	SaleID    int64     `json:"sale_id"`
	ExpiresAt time.Time `json:"expires_at"`
	IsUsed    bool      `json:"is_used"`
	CreatedAt time.Time `json:"created_at"`
}

type Purchase struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	ItemID       int64     `json:"item_id"`
	SaleID       int64     `json:"sale_id"`
	CheckoutCode string    `json:"checkout_code"`
	PurchaseTime time.Time `json:"purchase_time"`
	CreatedAt    time.Time `json:"created_at"`
}

type UserSaleSummary struct {
	UserID         string `json:"user_id"`
	SaleID         int64  `json:"sale_id"`
	ItemsPurchased int    `json:"items_purchased"`
}
