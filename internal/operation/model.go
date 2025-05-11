package operation

type OperationStatus int

const (
	StatusPending OperationStatus = iota
	StatusReady
	StatusDone
)

func (o OperationStatus) String() string {
	return []string{"Pending", "Ready", "Done"}[o]
}

type Operation struct {
	ID           string   `json:"id"`
	ExprID       string   `json:"expr_id"`
	Left         string   `json:"left"`
	Right        string   `json:"right"`
	Operator     string   `json:"operator"`
	Dependencies []string `json:"dependencies"`
	Result       float32  `json:"result"`
	Status       string   `json:"status"`
}
