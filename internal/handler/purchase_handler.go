package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"notcoin_contest/internal/service"
)

type PurchaseHandler struct {
	logger      *log.Logger
	saleService *service.SaleService
}

func NewPurchaseHandler(logger *log.Logger, saleService *service.SaleService) *PurchaseHandler {
	return &PurchaseHandler{
		logger:      logger,
		saleService: saleService,
	}
}

type PurchaseResponsePayload struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	ItemID  int64  `json:"item_id,omitempty"`
}

func (h *PurchaseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Printf("Method not allowed for /purchase: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "code query parameter is required", http.StatusBadRequest)
		return
	}

	purchasedItem, err := h.saleService.ProcessPurchase(r.Context(), code)
	if err != nil {
		var statusCode int
		var message string

		switch err {
		case service.ErrCheckoutCodeInvalid, service.ErrCheckoutCodeExpired, service.ErrCheckoutCodeAlreadyUsed:
			statusCode = http.StatusBadRequest
			message = err.Error()
		case service.ErrSaleNotActive:
			statusCode = http.StatusServiceUnavailable
			message = err.Error()
		case service.ErrItemNotFoundOrSold:
			statusCode = http.StatusConflict
			message = "Item is no longer available or already sold"
		case service.ErrUserLimitReached:
			statusCode = http.StatusForbidden
			message = err.Error()
		case service.ErrSaleLimitReached:
			statusCode = http.StatusConflict
			message = err.Error()
		case service.ErrPurchaseFailed:
			statusCode = http.StatusInternalServerError
			message = "Purchase processing failed due to an internal error"
		default:
			statusCode = http.StatusInternalServerError
			message = "An unexpected error occurred during purchase"
		}

		resp := PurchaseResponsePayload{Status: "failed", Message: message}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(resp)
		return
	}

	resp := PurchaseResponsePayload{
		Status:  "success",
		Message: "Item purchased successfully",
		ItemID:  purchasedItem.ID,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Printf("Error encoding purchase response: %v", err)
	}
}
