package service

import (
	"context"
	cRand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"time"

	"notcoin_contest/internal/config"
	"notcoin_contest/internal/models"
	"notcoin_contest/internal/store"
)

const (
	itemsPerSale = 10000
)

type SaleService struct {
	dbStore    *store.DBStore
	redisStore *store.RedisStore
	config     *config.Config
	logger     *log.Logger
}

func NewSaleService(logger *log.Logger, db *store.DBStore, redis *store.RedisStore, cfg *config.Config) *SaleService {
	return &SaleService{
		dbStore:    db,
		redisStore: redis,
		config:     cfg,
		logger:     logger,
	}
}

func (s *SaleService) ManageHourlySaleCycle(ctx context.Context) error {
	s.logger.Println("Starting new hourly sale cycle...")

	s.logger.Println("Deactivating all previously active sales...")
	if err := s.dbStore.DeactivateAllActiveSales(); err != nil {
		s.logger.Printf("Error deactivating active sales: %v", err)
	} else {
		s.logger.Println("Successfully deactivated all previously active sales.")
	}

	s.logger.Println("Creating new sale and items...")
	sale, items, err := s.CreateNewSaleAndItems()
	if err != nil {
		s.logger.Printf("Error creating new sale and items: %v", err)
		return fmt.Errorf("failed to create new sale and items: %w", err)
	}
	s.logger.Printf("Successfully created new sale ID %d with %d items. Sale active from %s to %s.",
		sale.ID, len(items), sale.StartTime.Format(time.RFC3339), sale.EndTime.Format(time.RFC3339))

	s.logger.Println("Hourly sale cycle completed successfully.")
	return nil
}

func (s *SaleService) CreateNewSaleAndItems() (*models.Sale, []models.Item, error) {
	now := time.Now()
	sale := &models.Sale{
		StartTime:  now,
		EndTime:    now.Add(s.config.SaleDuration),
		TotalItems: itemsPerSale,
		SoldItems:  0,
		IsActive:   true,
	}

	createdSale, err := s.dbStore.CreateSale(sale)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create sale in DB: %w", err)
	}

	items := make([]models.Item, 0, itemsPerSale)
	for i := 0; i < itemsPerSale; i++ {
		items = append(items, models.Item{
			SaleID:   createdSale.ID,
			Name:     fmt.Sprintf("Awesome Item #%d-%d", createdSale.ID, i+1),
			ImageURL: fmt.Sprintf("https://example.com/image/%d/%d.png", createdSale.ID, rand.Intn(1000)),
			IsSold:   false,
		})
	}

	createdItems, err := s.dbStore.CreateItemsBatch(items)
	if err != nil {
		s.logger.Printf("Failed to create items batch for sale ID %d: %v", createdSale.ID, err)
		if deactivateErr := s.dbStore.DeactivateSaleByID(createdSale.ID); deactivateErr != nil {
			s.logger.Printf("Additionally failed to deactivate sale ID %d after item creation failure: %v", createdSale.ID, deactivateErr)
		}
		return createdSale, nil, fmt.Errorf("failed to create items in DB: %w", err)
	}

	return createdSale, createdItems, nil
}

func (s *SaleService) GetCurrentActiveSale() (*models.Sale, error) {
	return s.dbStore.GetActiveSale()
}

const userMaxItemsPerSale = 10

var (
	ErrSaleNotActive           = errors.New("no active sale at the moment")
	ErrItemNotFoundOrSold      = errors.New("item not found, not part of active sale, or already sold")
	ErrUserLimitReached        = errors.New("user has reached the purchase limit for this sale")
	ErrCheckoutFailed          = errors.New("checkout processing failed")
	ErrCheckoutCodeInvalid     = errors.New("checkout code is invalid")
	ErrCheckoutCodeAlreadyUsed = errors.New("checkout code has already been used")
	ErrCheckoutCodeExpired     = errors.New("checkout code has expired")
	ErrSaleLimitReached        = errors.New("sale item limit reached")
	ErrPurchaseFailed          = errors.New("purchase failed")
)

func generateUniqueID(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := cRand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *SaleService) ProcessCheckout(ctx context.Context, userID string, itemID int64) (string, error) {
	activeSale, err := s.dbStore.GetActiveSale()
	if err != nil {
		return "", fmt.Errorf("failed to get active sale: %w", err)
	}
	if activeSale == nil {
		return "", ErrSaleNotActive
	}

	item, err := s.dbStore.GetItemForCheckout(itemID, activeSale.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get item details: %w", err)
	}
	if item == nil || item.IsSold {
		return "", ErrItemNotFoundOrSold
	}

	userPurchaseCount, err := s.dbStore.GetUserPurchaseCountForSale(userID, activeSale.ID)
	if err != nil {
		return "", fmt.Errorf("failed to get user purchase count: %w", err)
	}
	if userPurchaseCount >= userMaxItemsPerSale {
		return "", ErrUserLimitReached
	}

	checkoutCode, err := generateUniqueID(16)
	if err != nil {
		return "", fmt.Errorf("%w: failed to generate unique code: %v", ErrCheckoutFailed, err)
	}

	codeExpiryDuration := s.config.CodeTTLExpiry

	checkoutAttempt := &models.CheckoutAttempt{
		ID:        checkoutCode,
		UserID:    userID,
		ItemID:    itemID,
		SaleID:    activeSale.ID,
		ExpiresAt: time.Now().Add(codeExpiryDuration),
		IsUsed:    false,
	}

	if err := s.dbStore.CreateCheckoutAttempt(checkoutAttempt); err != nil {
		return "", fmt.Errorf("%w: failed to save checkout attempt: %v", ErrCheckoutFailed, err)
	}

	if err := s.redisStore.StoreCheckoutCode(ctx, checkoutAttempt, codeExpiryDuration); err != nil {
		s.logger.Printf("Warning: failed to store checkout code %s in Redis: %v\n", checkoutCode, err)
	}

	return checkoutCode, nil
}

func (s *SaleService) ProcessPurchase(ctx context.Context, code string) (*models.Item, error) {
	checkoutAttempt, err := s.getValidCheckoutAttempt(ctx, code)
	if err != nil {
		return nil, err
	}

	purchasedItem, err := s.dbStore.ExecutePurchaseTransaction(
		checkoutAttempt.UserID,
		checkoutAttempt.ItemID,
		checkoutAttempt.SaleID,
		checkoutAttempt.ID,
		userMaxItemsPerSale,
	)
	if err != nil {
		if errors.Is(err, store.ErrDBItemAlreadySold) {
			return nil, ErrItemNotFoundOrSold
		}
		if errors.Is(err, store.ErrDBSaleLimitReached) {
			return nil, ErrSaleLimitReached
		}
		if errors.Is(err, store.ErrDBUserPurchaseLimitReached) {
			return nil, ErrUserLimitReached
		}
		s.logger.Printf("Error during ExecutePurchaseTransaction for code %s: %v\n", code, err)
		return nil, ErrPurchaseFailed
	}

	if err := s.redisStore.DeleteCheckoutCode(ctx, code); err != nil {
		s.logger.Printf("Warning: failed to delete checkout code %s from Redis after successful purchase: %v\n", code, err)
	}

	return purchasedItem, nil
}

func (s *SaleService) getValidCheckoutAttempt(ctx context.Context, code string) (*models.CheckoutAttempt, error) {
	attempt, err := s.redisStore.GetCheckoutAttempt(ctx, code)
	if err != nil {
		s.logger.Printf("Redis GetCheckoutAttempt error for code %s: %v. Falling back to DB.\n", code, err)
	}

	if attempt == nil {
		s.logger.Printf("Code %s not found in Redis, checking DB.\n", code)
		attempt, err = s.dbStore.GetCheckoutAttemptByID(code)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrCheckoutCodeInvalid
			}
			return nil, fmt.Errorf("failed to query checkout attempt from DB: %w", err)
		}
		if attempt == nil {
			return nil, ErrCheckoutCodeInvalid
		}
	}

	if attempt.IsUsed {
		return nil, ErrCheckoutCodeAlreadyUsed
	}
	if time.Now().After(attempt.ExpiresAt) {
		return nil, ErrCheckoutCodeExpired
	}

	sale, err := s.dbStore.GetSaleByID(attempt.SaleID)
	if err != nil || sale == nil {
		return nil, ErrSaleNotActive
	}
	if !sale.IsActive || time.Now().After(sale.EndTime) || time.Now().Before(sale.StartTime) {
		return nil, ErrSaleNotActive
	}

	return attempt, nil
}
