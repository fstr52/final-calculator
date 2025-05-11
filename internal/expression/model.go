package expression

import (
	"fmt"
	"time"

	op "github.com/fstr52/final-calculator/internal/operation"
)

type ExpressionStatus int

const (
	StatusCreated ExpressionStatus = iota
	StatusInQueue
	StatusPlanned
	StatusComputing
	StatusDone
	StatusError
)

func (e ExpressionStatus) String() string {
	return []string{"Created", "In Queue", "Planned", "Computing", "Done", "Error"}[e]
}

type Expression struct {
	ID              string `json:"id"`
	UserID          string `json:"user_id"`
	Operations      []op.Operation
	RootOperationID string    `json:"root_operation_id"`
	Status          string    `json:"status"`
	Success         bool      `json:"success"`
	Result          float32   `json:"result"`
	Error           string    `json:"error"`
	Expression      string    `json:"expression"`
	CreatedAt       time.Time `json:"created_at"`
}

func (e *Expression) String() string {
	return fmt.Sprintf("Expression{Id: %s, Success: %t, Result: %.2f, Error: %v, Status: %s, Operations: %d}",
		e.ID, e.Success, e.Result, e.Error, e.Status, len(e.Operations))
}
