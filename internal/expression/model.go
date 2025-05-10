package expression

import "time"

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
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Status     string    `json:"status"`
	Success    bool      `json:"success"`
	Result     float32   `json:"result"`
	Error      string    `json:"error"`
	Expression string    `json:"expression"`
	CreatedAt  time.Time `json:"created_at"`
}
