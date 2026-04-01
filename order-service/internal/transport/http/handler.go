package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"order-service/internal/usecase"
)

type OrderHandler struct {
	usecase *usecase.OrderUseCase
}

func NewOrderHandler(router *gin.Engine, uc *usecase.OrderUseCase) {
	handler := &OrderHandler{usecase: uc}

	router.POST("/orders", handler.CreateOrder)
	router.GET("/orders/:id", handler.GetOrderByID)
	router.PATCH("/orders/:id/cancel", handler.CancelOrder)
}

type CreateOrderRequest struct {
	CustomerID string `json:"customer_id" binding:"required"`
	ItemName   string `json:"item_name" binding:"required"`
	Amount     int64  `json:"amount" binding:"required"`
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")

	order, err := h.usecase.CreateOrder(c.Request.Context(), req.CustomerID, req.ItemName, req.Amount, idempotencyKey)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidAmount) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, usecase.ErrPaymentServiceUnavailable) && order != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":      err.Error(),
				"order_id":   order.ID,
				"status":     order.Status,
				"created_at": order.CreatedAt,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":          order.ID,
		"customer_id": order.CustomerID,
		"item_name":   order.ItemName,
		"amount":      order.Amount,
		"status":      order.Status,
		"created_at":  order.CreatedAt,
	})
}

func (h *OrderHandler) GetOrderByID(c *gin.Context) {
	id := c.Param("id")
	order, err := h.usecase.GetOrderByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          order.ID,
		"customer_id": order.CustomerID,
		"item_name":   order.ItemName,
		"amount":      order.Amount,
		"status":      order.Status,
		"created_at":  order.CreatedAt,
	})
}

func (h *OrderHandler) CancelOrder(c *gin.Context) {
	id := c.Param("id")
	order, err := h.usecase.CancelOrder(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, usecase.ErrOrderCannotBeCancelled) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if order == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":          order.ID,
		"customer_id": order.CustomerID,
		"item_name":   order.ItemName,
		"amount":      order.Amount,
		"status":      order.Status,
		"created_at":  order.CreatedAt,
	})
}
