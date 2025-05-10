package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fstr52/final-calculator/internal/logger"
	"github.com/fstr52/final-calculator/internal/parser"
	pr "github.com/fstr52/final-calculator/internal/proto"
	"github.com/google/uuid"
)

type Orchestrator struct {
	pr.OrchestratorServiceServer
	logger logger.Logger

	expressionQueue  []*Expression
	doneCache        map[string]float32
	pendingOps       map[string]OperationRequest
	expressionByRoot map[string]*Expression
	toSend           []OperationRequest

	cacheMu sync.RWMutex
	exprMu  sync.Mutex
	sendMu  sync.Mutex
}

func NewOrchestrator(logger logger.Logger) *Orchestrator {
	return &Orchestrator{
		logger:           logger,
		expressionQueue:  make([]*Expression, 0),
		doneCache:        make(map[string]float32),
		pendingOps:       make(map[string]OperationRequest),
		expressionByRoot: make(map[string]*Expression),
		toSend:           make([]OperationRequest, 0),
	}
}

type ExpressionStatus int

const (
	StatusCreated ExpressionStatus = iota
	StatusInQueue
	StatusComputing
	StatusDone
	StatusError
)

func (e ExpressionStatus) String() string {
	return []string{"Created", "In Queue", "Computing", "Done", "Error"}[e]
}

type Expression struct {
	Id         string
	Operations []OperationRequest
	Success    bool
	Result     float32
	Error      error
	Status     ExpressionStatus
}

func (e *Expression) String() string {
	return fmt.Sprintf("Expression{Id: %s, Success: %t, Result: %.2f, Error: %v, Status: %s, Operations: %d}",
		e.Id, e.Success, e.Result, e.Error, e.Status.String(), len(e.Operations))
}

type OperationRequest struct {
	Id           string
	Left         string
	Operator     string
	Right        string
	HasTask      bool
	Dependencies []string
	ExprId       string
}

func (o *Orchestrator) Run(ctx context.Context) {
	o.logger.Debug("Started Run for Orchestrator")
	go o.startScheduler(ctx)
}

func (o *Orchestrator) NewExpression(input string) (*Expression, error) {
	o.logger.Debug("Started NewExpression",
		"input", input)
	p := parser.NewParser(input, o.logger)
	ast, err := p.ParseExpr()
	if err != nil {
		o.logger.Warn("Failed to ParseExpr",
			"error", err)
		return nil, err
	}

	expr := &Expression{}
	expr.Status = StatusCreated

	rootId := o.planOperations(expr, ast)
	expr.Id = rootId

	o.exprMu.Lock()
	o.expressionQueue = append(o.expressionQueue, expr)
	o.exprMu.Unlock()
	expr.Status = StatusInQueue

	o.logger.Debug("Successfully created new expression",
		"Expression", expr.String())
	return expr, nil
}

func (o *Orchestrator) planOperations(expr *Expression, ast parser.Expr) string {
	o.logger.Debug("Started planOperations",
		"expr", expr.String(),
		"ast", fmt.Sprintf("%+v", ast))
	switch n := ast.(type) {
	case *parser.Number:
		o.logger.Debug("Parsed parser.Number")
		id := uuid.NewString()
		o.cacheMu.Lock()
		o.doneCache[id] = ast.Eval()
		o.cacheMu.Unlock()
		o.logger.Debug("Number",
			"id", id)
		return id
	case *parser.Binary:
		o.logger.Debug("Parsed parser.Binary")
		leftId := o.planOperations(expr, n.Left)
		rightId := o.planOperations(expr, n.Right)

		id := uuid.NewString()

		op := OperationRequest{
			Id:           id,
			Left:         leftId,
			Operator:     n.Symbol.Value,
			Right:        rightId,
			Dependencies: []string{leftId, rightId},
			ExprId:       expr.Id,
		}

		expr.Operations = append(expr.Operations, op)

		o.logger.Debug("Binary",
			"id", id)
		return id
	}
	return ""
}

func (o *Orchestrator) startScheduler(ctx context.Context) {
	o.logger.Debug("Started Scheduler")
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.logger.Debug("Scheduler stoped gracefully")
			return
		case <-ticker.C:
			o.nextExpr()
			o.processPending()
		}
	}
}

func (o *Orchestrator) nextExpr() {
	if len(o.expressionQueue) == 0 {
		return
	}

	o.exprMu.Lock()
	expr := o.expressionQueue[0]
	o.expressionQueue = o.expressionQueue[1:]
	o.exprMu.Unlock()

	for _, op := range expr.Operations {
		ready := true

		for _, dep := range op.Dependencies {
			o.cacheMu.RLock()
			done := false
			if _, exists := o.doneCache[dep]; exists {
				done = true
			}
			o.cacheMu.RUnlock()

			if !done {
				o.logger.Debug("dep not done", "dep", dep)
				ready = false
				break
			}
		}

		if ready {
			o.enqueue(op)
		} else {
			o.pendingOps[op.Id] = op
		}
	}
}

func (o *Orchestrator) processPending() {
	if len(o.pendingOps) == 0 {
		return
	}

	for id, op := range o.pendingOps {
		allReady := true

		for _, dep := range op.Dependencies {
			o.cacheMu.RLock()
			if _, done := o.doneCache[dep]; !done {
				o.logger.Debug("dep not done",
					"dep", dep)
				allReady = false
				break
			}
			o.cacheMu.RUnlock()
		}

		if allReady {
			o.logger.Debug("allReady",
				"op", op)
			o.enqueue(op)
			delete(o.pendingOps, id)
		}
	}
}

func (o *Orchestrator) enqueue(op OperationRequest) {
	o.sendMu.Lock()
	o.toSend = append(o.toSend, op)
	o.sendMu.Unlock()
}

func (o *Orchestrator) GetTask(ctx context.Context, info *pr.WorkerInfo) (*pr.Task, error) {
	o.logger.Debug("Started GetTask",
		"workerId", info.WorkerId)

	if len(o.toSend) == 0 {
		o.logger.Debug("toSend len == 0")
		return &pr.Task{HasTask: false}, nil
	}

	o.sendMu.Lock()
	op := o.toSend[0]
	o.toSend = o.toSend[1:]
	o.sendMu.Unlock()

	o.cacheMu.RLock()
	left := o.doneCache[op.Left]
	right := o.doneCache[op.Right]
	o.cacheMu.RUnlock()

	task := &pr.Task{
		TaskId:   op.Id,
		Left:     left,
		Operator: op.Operator,
		Right:    right,
		HasTask:  true,
		ExprId:   op.ExprId,
	}

	o.logger.Debug("Sending task",
		"task", fmt.Sprintf("%+v", task))

	return task, nil
}

func (o *Orchestrator) SubmitResult(ctx context.Context, res *pr.TaskResult) (*pr.Ack, error) {
	o.logger.Debug("Started SubmitResult",
		"res", fmt.Sprintf("%+v", res))
	ack := &pr.Ack{Accepted: true}

	if res.Error != "" {
		o.logger.Debug("res error",
			"error", res.Error)
		//тут надо в бд ставить ошибку выражению по res.ExprId

		return ack, nil
	}

	o.cacheMu.Lock()
	if _, ok := o.doneCache[res.TaskId]; !ok {
		o.logger.Error("Received task not found by ID",
			"ID", res.TaskId)
		panic("received task not found by ID")
		//return ack, fmt.Errorf("received task not found by ID")
	}
	o.doneCache[res.TaskId] = res.Result
	o.cacheMu.Unlock()

	if expr, ok := o.expressionByRoot[res.TaskId]; ok {
		expr.Status = StatusDone
	}

	o.logger.Debug("SubmitResult finished")
	return ack, nil
}
