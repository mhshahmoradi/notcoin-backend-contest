package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"notcoin_contest/internal/models"

	_ "github.com/lib/pq"
)

var (
	ErrDBItemAlreadySold          = errors.New("database: item already sold")
	ErrDBSaleLimitReached         = errors.New("database: sale item limit reached")
	ErrDBUserPurchaseLimitReached = errors.New("database: user purchase limit for this sale reached")
)

type DBStore struct {
	DB *sql.DB
}

func NewDBStore(db *sql.DB) *DBStore {
	return &DBStore{DB: db}
}

func ConnectDB(driver, dataSourceName string) (*sql.DB, error) {
	db, err := sql.Open(driver, dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

func RunMigrations(db *sql.DB, migrationsDir string) error {
	if migrationsDir == "" {
		return fmt.Errorf("migrations directory not specified")
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrationFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			migrationFiles = append(migrationFiles, entry.Name())
		}
	}
	sort.Strings(migrationFiles)

	if len(migrationFiles) == 0 {
		fmt.Println("No migration files found.")
		return nil
	}

	fmt.Printf("Found migration files: %v\n", migrationFiles)

	for _, fileName := range migrationFiles {
		filePath := filepath.Join(migrationsDir, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", fileName, err)
		}

		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", fileName, err)
		}
		fmt.Printf("Applied migration: %s\n", fileName)
	}
	fmt.Println("All migrations applied successfully.")
	return nil
}

func (s *DBStore) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func (s *DBStore) CreateSale(sale *models.Sale) (*models.Sale, error) {
	query := `
        INSERT INTO sales (start_time, end_time, total_items, sold_items, is_active)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at, updated_at`

	err := s.DB.QueryRow(
		query,
		sale.StartTime,
		sale.EndTime,
		sale.TotalItems,
		sale.SoldItems,
		sale.IsActive,
	).Scan(&sale.ID, &sale.CreatedAt, &sale.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create sale: %w", err)
	}
	return sale, nil
}

func (s *DBStore) CreateItemsBatch(items []models.Item) ([]models.Item, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("no items to create")
	}

	tx, err := s.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
        INSERT INTO items (sale_id, name, image_url, is_sold)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at, updated_at`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	createdItems := make([]models.Item, len(items))
	for i, item := range items {
		err := stmt.QueryRow(item.SaleID, item.Name, item.ImageURL, item.IsSold).Scan(
			&createdItems[i].ID, &createdItems[i].CreatedAt, &createdItems[i].UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to insert item %d: %w", i, err)
		}
		createdItems[i].SaleID = item.SaleID
		createdItems[i].Name = item.Name
		createdItems[i].ImageURL = item.ImageURL
		createdItems[i].IsSold = item.IsSold
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return createdItems, nil
}

func (s *DBStore) GetActiveSale() (*models.Sale, error) {
	query := `
        SELECT id, start_time, end_time, total_items, sold_items, is_active, created_at, updated_at
        FROM sales
        WHERE is_active = TRUE AND NOW() BETWEEN start_time AND end_time
        ORDER BY start_time DESC
        LIMIT 1`

	sale := &models.Sale{}
	err := s.DB.QueryRow(query).Scan(
		&sale.ID,
		&sale.StartTime,
		&sale.EndTime,
		&sale.TotalItems,
		&sale.SoldItems,
		&sale.IsActive,
		&sale.CreatedAt,
		&sale.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active sale: %w", err)
	}
	return sale, nil
}

func (s *DBStore) GetItemForCheckout(itemID int64, saleID int64) (*models.Item, error) {
	query := `
        SELECT id, sale_id, name, image_url, is_sold, created_at, updated_at
        FROM items
        WHERE id = $1 AND sale_id = $2 AND is_sold = FALSE`

	item := &models.Item{}
	err := s.DB.QueryRow(query, itemID, saleID).Scan(
		&item.ID,
		&item.SaleID,
		&item.Name,
		&item.ImageURL,
		&item.IsSold,
		&item.CreatedAt,
		&item.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get item for checkout: %w", err)
	}
	return item, nil
}

func (s *DBStore) GetUserPurchaseCountForSale(userID string, saleID int64) (int, error) {
	query := `
        SELECT items_purchased
        FROM user_sale_limits
        WHERE user_id = $1 AND sale_id = $2`

	var count int
	err := s.DB.QueryRow(query, userID, saleID).Scan(&count)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get user purchase count: %w", err)
	}
	return count, nil
}

func (s *DBStore) CreateCheckoutAttempt(attempt *models.CheckoutAttempt) error {
	query := `
        INSERT INTO checkout_attempts (id, user_id, item_id, sale_id, expires_at, is_used, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW())
        RETURNING created_at`

	err := s.DB.QueryRow(
		query,
		attempt.ID,
		attempt.UserID,
		attempt.ItemID,
		attempt.SaleID,
		attempt.ExpiresAt,
		attempt.IsUsed,
	).Scan(&attempt.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create checkout attempt: %w", err)
	}
	return nil
}

func (s *DBStore) GetCheckoutAttemptByID(code string) (*models.CheckoutAttempt, error) {
	query := `
        SELECT id, user_id, item_id, sale_id, expires_at, is_used, created_at
        FROM checkout_attempts
        WHERE id = $1`
	attempt := &models.CheckoutAttempt{}
	err := s.DB.QueryRow(query, code).Scan(
		&attempt.ID,
		&attempt.UserID,
		&attempt.ItemID,
		&attempt.SaleID,
		&attempt.ExpiresAt,
		&attempt.IsUsed,
		&attempt.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get checkout attempt: %w", err)
	}
	return attempt, nil
}

func (s *DBStore) GetSaleByID(saleID int64) (*models.Sale, error) {
	query := `
        SELECT id, start_time, end_time, total_items, sold_items, is_active, created_at, updated_at
        FROM sales
        WHERE id = $1`
	sale := &models.Sale{}
	err := s.DB.QueryRow(query, saleID).Scan(
		&sale.ID, &sale.StartTime, &sale.EndTime, &sale.TotalItems,
		&sale.SoldItems, &sale.IsActive, &sale.CreatedAt, &sale.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get sale by ID: %w", err)
	}
	return sale, nil
}

func (s *DBStore) ExecutePurchaseTransaction(userID string, itemID int64, saleID int64, checkoutCode string, userItemLimitPerSale int) (*models.Item, error) {
	tx, err := s.DB.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var item models.Item
	itemQuery := `SELECT id, sale_id, name, image_url, is_sold FROM items WHERE id = $1 AND sale_id = $2 FOR UPDATE`
	err = tx.QueryRow(itemQuery, itemID, saleID).Scan(&item.ID, &item.SaleID, &item.Name, &item.ImageURL, &item.IsSold)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("item not found")
		}
		return nil, fmt.Errorf("failed to lock item: %w", err)
	}
	if item.IsSold {
		return nil, ErrDBItemAlreadySold
	}

	var currentSale models.Sale
	saleQuery := `SELECT id, total_items, sold_items, is_active, end_time FROM sales WHERE id = $1 FOR UPDATE`
	err = tx.QueryRow(saleQuery, saleID).Scan(&currentSale.ID, &currentSale.TotalItems, &currentSale.SoldItems, &currentSale.IsActive, &currentSale.EndTime)
	if err != nil {
		return nil, fmt.Errorf("failed to lock sale: %w", err)
	}
	if !currentSale.IsActive || time.Now().After(currentSale.EndTime) {
		return nil, fmt.Errorf("sale is not active or has ended")
	}
	if currentSale.SoldItems >= currentSale.TotalItems {
		return nil, ErrDBSaleLimitReached
	}

	var userPurchaseCount int
	userLimitQuery := `SELECT items_purchased FROM user_sale_limits WHERE user_id = $1 AND sale_id = $2 FOR UPDATE`
	err = tx.QueryRow(userLimitQuery, userID, saleID).Scan(&userPurchaseCount)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to check user purchase limit: %w", err)
	}
	if userPurchaseCount >= userItemLimitPerSale {
		return nil, ErrDBUserPurchaseLimitReached
	}

	_, err = tx.Exec(`UPDATE items SET is_sold = TRUE WHERE id = $1`, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to mark item as sold: %w", err)
	}

	_, err = tx.Exec(`UPDATE sales SET sold_items = sold_items + 1 WHERE id = $1`, saleID)
	if err != nil {
		return nil, fmt.Errorf("failed to increment sale sold_items: %w", err)
	}

	_, err = tx.Exec(`
        INSERT INTO purchases (user_id, item_id, sale_id, checkout_code, purchased_at)
        VALUES ($1, $2, $3, $4, NOW())`, userID, itemID, saleID, checkoutCode)
	if err != nil {
		return nil, fmt.Errorf("failed to record purchase: %w", err)
	}

	_, err = tx.Exec(`
        INSERT INTO user_sale_limits (user_id, sale_id, items_purchased)
        VALUES ($1, $2, 1)
        ON CONFLICT (user_id, sale_id)
        DO UPDATE SET items_purchased = user_sale_limits.items_purchased + 1
        WHERE user_sale_limits.items_purchased < $3`, userID, saleID, userItemLimitPerSale)
	if err != nil {
		return nil, fmt.Errorf("failed to update user sale limits: %w", err)
	}


	_, err = tx.Exec(`UPDATE checkout_attempts SET is_used = TRUE WHERE id = $1`, checkoutCode)
	if err != nil {
		return nil, fmt.Errorf("failed to mark checkout code as used: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	item.IsSold = true
	return &item, nil
}

func (s *DBStore) DeactivateAllActiveSales() error {
	_, err := s.DB.Exec(`UPDATE sales SET is_active = FALSE WHERE is_active = TRUE`)
	if err != nil {
		return fmt.Errorf("failed to deactivate all active sales: %w", err)
	}
	return nil
}

func (s *DBStore) DeactivateSaleByID(saleID int64) error {
	_, err := s.DB.Exec(`UPDATE sales SET is_active = FALSE WHERE id = $1`, saleID)
	if err != nil {
		return fmt.Errorf("failed to deactivate sale by ID: %w", err)
	}
	return nil
}
