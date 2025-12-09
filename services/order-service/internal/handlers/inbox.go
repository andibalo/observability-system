package handlers

import (
	"net/http"

	"observability-system/shared/logger"
	"order-service/internal/inbox"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type InboxHandler struct {
	logger     logger.Logger
	inboxStore *inbox.InboxStore
}

func NewInboxHandler(log logger.Logger, inboxStore *inbox.InboxStore) *InboxHandler {
	return &InboxHandler{
		logger:     log,
		inboxStore: inboxStore,
	}
}

func (h *InboxHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"service": "order-service",
	})
}

func (h *InboxHandler) CreateInboxMessage(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		EventType string                 `json:"event_type" binding:"required"`
		Payload   map[string]interface{} `json:"payload" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.ErrorCtx(ctx, "Invalid request body",
			logger.Err(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	messageID := uuid.New().String()

	h.logger.InfoCtx(ctx, "Creating inbox message",
		logger.String("message_id", messageID),
		logger.String("event_type", req.EventType))

	err := h.inboxStore.Save(ctx, messageID, req.EventType, req.Payload)
	if err != nil {
		h.logger.ErrorCtx(ctx, "Failed to save inbox message",
			logger.Err(err),
			logger.String("message_id", messageID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save message",
			"details": err.Error(),
		})
		return
	}

	h.logger.InfoCtx(ctx, "Inbox message created successfully",
		logger.String("message_id", messageID))

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Inbox message created successfully",
		"message_id": messageID,
		"event_type": req.EventType,
		"request_id": logger.GetRequestIDFromGin(c),
	})
}

func (h *InboxHandler) GetInboxMessages(c *gin.Context) {
	ctx := c.Request.Context()

	h.logger.InfoCtx(ctx, "Fetching inbox messages")

	messages, err := h.inboxStore.GetAll(ctx)
	if err != nil {
		h.logger.ErrorCtx(ctx, "Failed to fetch inbox messages",
			logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch messages",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count":    len(messages),
		"messages": messages,
	})
}
