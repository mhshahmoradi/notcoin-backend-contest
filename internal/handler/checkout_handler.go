package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"notcoin_contest/internal/service"
)

type CheckoutHandler struct {
	logger      *log.Logger
	saleService *service.SaleService
}

func NewCheckoutHandler(logger *log.Logger, saleService *service.SaleService) *CheckoutHandler {
	return &CheckoutHandler{
		logger:      logger,
		saleService: saleService,
	}
}

type CheckoutResponsePayload struct {
	Code string `json:"code"`
}

func (h *CheckoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.logger.Printf("Method not allowed for /checkout: %s", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	itemIDStr := r.URL.Query().Get("id")

	if userID == "" {
		http.Error(w, "user_id query parameter is required", http.StatusBadRequest)
		return
	}
	if itemIDStr == "" {
		http.Error(w, "id query parameter is required", http.StatusBadRequest)
		return
	}

	itemID, err := strconv.ParseInt(itemIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid item id format", http.StatusBadRequest)
		return
	}

	code, err := h.saleService.ProcessCheckout(r.Context(), userID, itemID)
	if err != nil {
		switch err {
		case service.ErrSaleNotActive:
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		case service.ErrItemNotFoundOrSold:
			http.Error(w, err.Error(), http.StatusNotFound)
		case service.ErrUserLimitReached:
			http.Error(w, err.Error(), http.StatusForbidden)
		case service.ErrCheckoutFailed:
			http.Error(w, "Internal server error during checkout", http.StatusInternalServerError)
		default:
			http.Error(w, "An unexpected error occurred", http.StatusInternalServerError)
		}
		return
	}

	resp := CheckoutResponsePayload{Code: code}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Printf("Error encoding checkout response: %v", err)
	}
}
