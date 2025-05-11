package orchestrator

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/fstr52/final-calculator/internal/expression"
	"github.com/gin-gonic/gin"
)

type sendStruct struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Success    bool      `json:"success"`
	Result     float64   `json:"result"`
	Error      string    `json:"error,omitempty"`
	Expression string    `json:"expression"`
	CreatedAt  time.Time `json:"created_at"`
}

func (o *Orchestrator) CalculateHandler(c *gin.Context) {
	var userRequest struct {
		Expression string `json:"expression"`
	}

	if err := c.ShouldBindJSON(&userRequest); err != nil {
		o.logger.Error("Failed to decode request body",
			"error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	o.logger.Info("Processing calculation request",
		"expression", userRequest.Expression)

	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No userID in token"})
		return
	}

	userIDString, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Can't convert userID to string"})
		return
	}

	expr, err := o.NewExpression(c.Request.Context(), userRequest.Expression, userIDString)
	if err != nil {
		o.logger.Error("Failed to prepare input",
			"expression", userRequest.Expression,
			"error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expression"})
		return
	}

	response := gin.H{
		"id":     expr.ID,
		"status": expr.Status,
	}

	o.logger.Info("Calculation request processed successfully",
		"expression_id", expr.ID,
		"status", expr.Status)

	c.JSON(http.StatusOK, response)
}

func (o *Orchestrator) ExpressionsHandler(c *gin.Context) {
	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No userID in token"})
		return
	}

	userIDString, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Can't convert userID to string"})
		return
	}

	expressions, err := o.exprStorage.FindAllByUser(c.Request.Context(), userIDString)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err})
		return
	}

	sort.Slice(expressions, func(i, j int) bool {
		return expressions[i].CreatedAt.Unix() < expressions[j].CreatedAt.Unix()
	})

	sendExprs := ConvertExpressions(expressions)
	c.JSON(http.StatusOK, gin.H{"expressions": sendExprs})
}

func ConvertExpressions(expressions []expression.Expression) []sendStruct {
	sendSlice := make([]sendStruct, len(expressions))
	for i, expr := range expressions {
		sendSlice[i] = sendStruct{
			ID:         expr.ID,
			Status:     expr.Status,
			Success:    expr.Success,
			Result:     expr.Result,
			Error:      expr.Error,
			Expression: expr.Expression,
			CreatedAt:  expr.CreatedAt,
		}
	}
	return sendSlice
}

func (o *Orchestrator) ExpressionByIDHandler(c *gin.Context) {
	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No userID in token"})
		return
	}

	userIDString, ok := userID.(string)
	if !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Can't convert userID to string"})
		return
	}

	url := c.Request.URL.Path
	parts := strings.Split(url, "/")
	if len(parts) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL format"})
		return
	}
	exprIDpart := parts[len(parts)-1]
	exprID := strings.TrimPrefix(exprIDpart, ":")

	expr, err := o.exprStorage.FindOne(c.Request.Context(), exprID)
	if err != nil {
		o.logger.Error("Failed to find expression",
			"id", exprID,
			"error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Expression not found"})
		return
	}

	if expr.UserID != userIDString {
		o.logger.Warn("Unauthorized access attempt to expression",
			"expression_id", exprID,
			"requested_by", userIDString,
			"owned_by", expr.UserID)
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	sendExpr := sendStruct{
		ID:         expr.ID,
		Status:     expr.Status,
		Success:    expr.Success,
		Result:     expr.Result,
		Error:      expr.Error,
		Expression: expr.Expression,
		CreatedAt:  expr.CreatedAt,
	}

	c.JSON(http.StatusOK, gin.H{"expression": sendExpr})
}
