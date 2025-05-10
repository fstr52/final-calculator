package orchestrator

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

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

	expr, err := o.NewExpression(userRequest.Expression)
	if err != nil {
		o.logger.Error("Failed to prepare input",
			"expression", userRequest.Expression,
			"error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expression"})
		return
	}

	response := gin.H{
		"id":     expr.Id,
		"status": expr.Status,
	}

	o.logger.Info("Calculation request processed successfully",
		"expression_id", expr.Id,
		"status", expr.Status.String())

	c.JSON(http.StatusOK, response)
}
