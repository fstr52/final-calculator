package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/fstr52/final-calculator/internal/db/postgresql"
	ex "github.com/fstr52/final-calculator/internal/expression"
	exDB "github.com/fstr52/final-calculator/internal/expression/db"
	"github.com/fstr52/final-calculator/internal/logger"
	op "github.com/fstr52/final-calculator/internal/operation"
	opDB "github.com/fstr52/final-calculator/internal/operation/db"
	"github.com/fstr52/final-calculator/internal/parser"
	pr "github.com/fstr52/final-calculator/internal/proto"
	"github.com/google/uuid"
)

type Orchestrator struct {
	pr.OrchestratorServiceServer
	logger logger.Logger

	expressionQueue  []*ex.Expression
	doneCache        map[string]float32
	pendingOps       map[string]op.Operation
	expressionByRoot map[string]*ex.Expression
	toSend           []op.Operation
	exprStorage      ex.Storage
	opStorage        op.Storage

	cacheMu sync.RWMutex
	exprMu  sync.Mutex
	sendMu  sync.Mutex
}

func NewOrchestrator(logger logger.Logger, client postgresql.Client) *Orchestrator {
	return &Orchestrator{
		logger:           logger,
		expressionQueue:  make([]*ex.Expression, 0),
		doneCache:        make(map[string]float32),
		pendingOps:       make(map[string]op.Operation),
		expressionByRoot: make(map[string]*ex.Expression),
		toSend:           make([]op.Operation, 0),
		exprStorage:      exDB.NewStorage(client, logger),
		opStorage:        opDB.NewStorage(client, logger),
	}
}

func (o *Orchestrator) Run(ctx context.Context) {
	o.logger.Debug("Started Run for Orchestrator")

	o.restoreState(ctx)
	go o.startTTLWatcher(ctx)

	go o.startScheduler(ctx)
}

func (o *Orchestrator) restoreState(ctx context.Context) {
	o.logger.Info("Restoring state from DB...")

	exprs, err := o.exprStorage.GetUnfinishedExpressions(ctx)
	if err != nil {
		o.logger.Error("Failed to load unfinished expressions", "error", err)
		return
	}

	for _, expr := range exprs {
		o.logger.Debug("Restoring expression", "id", expr.ID)

		ops, err := o.opStorage.GetOperationsByExpression(ctx, expr.ID)
		if err != nil {
			o.logger.Error("Failed to get operations", "exprID", expr.ID, "error", err)
			continue
		}
		expr.Operations = ops

		o.exprMu.Lock()
		o.expressionQueue = append(o.expressionQueue, expr)
		o.exprMu.Unlock()

		o.expressionByRoot[expr.RootOperationID] = expr

		for _, op := range ops {
			if op.Status == "done" {
				o.cacheMu.Lock()
				o.doneCache[op.ID] = op.Result
				o.cacheMu.Unlock()
				continue
			}

			ready := true
			o.cacheMu.RLock()
			for _, dep := range op.Dependencies {
				if _, ok := o.doneCache[dep]; !ok {
					ready = false
					break
				}
			}
			o.cacheMu.RUnlock()

			if ready {
				o.enqueue(op)
			} else {
				o.pendingOps[op.ID] = op
			}
		}
	}

	o.logger.Info("State restored successfully")
}

func (o *Orchestrator) NewExpression(input string, userID string) (*ex.Expression, error) {
	o.logger.Debug("Started NewExpression",
		"input", input)
	p := parser.NewParser(input, o.logger)
	ast, err := p.ParseExpr()
	if err != nil {
		o.logger.Warn("Failed to ParseExpr",
			"error", err)
		return nil, err
	}

	expr := &ex.Expression{
		ID:         uuid.NewString(),
		UserID:     userID,
		Status:     ex.StatusCreated.String(),
		Expression: input,
	}

	if err = o.exprStorage.Create(context.TODO(), *expr); err != nil {
		return nil, err
	}

	rootId := o.planOperations(expr, ast)
	expr.RootOperationID = rootId

	o.exprMu.Lock()
	o.expressionQueue = append(o.expressionQueue, expr)
	o.exprMu.Unlock()
	expr.Status = ex.StatusInQueue.String()
	o.expressionByRoot[rootId] = expr

	if err := o.UpdateExpression(context.TODO(), *expr); err != nil {
		return nil, err
	}

	o.logger.Debug("Successfully created new expression",
		"Expression", expr.String())
	return expr, nil
}

func (o *Orchestrator) planOperations(expr *ex.Expression, ast parser.Expr) string {
	switch n := ast.(type) {
	case *parser.Number:
		id := uuid.NewString()
		o.cacheMu.Lock()
		o.doneCache[id] = ast.Eval()
		o.cacheMu.Unlock()
		return id
	case *parser.Binary:
		leftId := o.planOperations(expr, n.Left)
		rightId := o.planOperations(expr, n.Right)

		// Проверка на пустой id (!!!)
		if leftId == "" || rightId == "" {
			o.logger.Error("planOperations got empty left/rightId",
				"leftId", leftId,
				"rightId", rightId,
				"op", n.Symbol.Value)
			return "" // или panic, или лучше error!
		}

		id := uuid.NewString()
		oper := op.Operation{
			ID:           id,
			Left:         leftId,
			Operator:     n.Symbol.Value,
			Right:        rightId,
			Dependencies: []string{leftId, rightId},
			ExprID:       expr.ID,
		}
		oper.Status = op.StatusPending.String()
		if err := o.opStorage.Create(context.TODO(), oper); err != nil {
			o.logger.Error("Failed to save operation", "error", err)
		}

		expr.Operations = append(expr.Operations, oper)
		return id

	default:
		o.logger.Error("planOperations: unknown AST node type",
			"type", fmt.Sprintf("%T", ast))
		panic("unknown AST node type")
	}
}

func (o *Orchestrator) startTTLWatcher(ctx context.Context) {
	const ttl = 5 * time.Minute
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			o.logger.Info("TTL watcher stopped")
			return
		case <-ticker.C:
			now := time.Now()

			exprs, err := o.exprStorage.GetUnfinishedExpressions(ctx)
			if err != nil {
				o.logger.Error("Failed to load unfinished expressions for TTL", "error", err)
				continue
			}

			for _, expr := range exprs {
				age := now.Sub(expr.CreatedAt)
				if age > ttl {
					o.logger.Warn("Expression timed out", "id", expr.ID, "age", age)

					expr.Status = ex.StatusError.String()
					expr.Success = false
					expr.Error = "expression timed out"

					if err := o.UpdateExpression(ctx, *expr); err != nil {
						o.logger.Error("Failed to mark expression as timed out", "id", expr.ID, "error", err)
					}
				}
			}
		}
	}
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
			o.pendingOps[op.ID] = op
		}
	}
}

func (o *Orchestrator) processPending() {
	if len(o.pendingOps) == 0 {
		return
	}

	var toDelete []string

	for id, op := range o.pendingOps {
		allReady := true
		for _, dep := range op.Dependencies {
			o.cacheMu.RLock()
			if _, done := o.doneCache[dep]; !done {
				o.logger.Debug("dep not done", "dep", dep)
				allReady = false
				o.cacheMu.RUnlock()
				break
			}
			o.cacheMu.RUnlock()
		}
		if allReady {
			o.logger.Debug("allReady", "op", op)
			o.enqueue(op)
			toDelete = append(toDelete, id)
		}
	}
	for _, id := range toDelete {
		delete(o.pendingOps, id)
	}
}

func (o *Orchestrator) enqueue(op op.Operation) {
	o.sendMu.Lock()
	o.toSend = append(o.toSend, op)
	o.sendMu.Unlock()
}

func (o *Orchestrator) GetTask(ctx context.Context, info *pr.WorkerInfo) (*pr.Task, error) {
	o.sendMu.Lock()
	defer o.sendMu.Unlock()

	if len(o.toSend) == 0 {
		return &pr.Task{HasTask: false}, nil
	}

	o.logger.Debug("Started GetTask with toSend len > 0",
		"workerId", info.WorkerId)

	op := o.toSend[0]
	o.toSend = o.toSend[1:]

	o.cacheMu.RLock()
	left := o.doneCache[op.Left]
	right := o.doneCache[op.Right]
	o.cacheMu.RUnlock()

	task := &pr.Task{
		TaskId:   op.ID,
		Left:     left,
		Operator: op.Operator,
		Right:    right,
		HasTask:  true,
		ExprId:   op.ExprID,
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
		if err := o.ErrorExpression(ctx, res.ExprId, res.Error); err != nil {
			return ack, err
		}

		return ack, nil
	}

	o.cacheMu.Lock()
	o.doneCache[res.TaskId] = res.Result
	o.cacheMu.Unlock()

	if err := o.opStorage.UpdateOperationResult(ctx, res.TaskId, res.Result, op.StatusDone.String()); err != nil {
		o.logger.Error("Failed to update operation result", "error", err)
	}

	o.logger.Debug("Trying to find expression by root",
		"taskID", res.TaskId,
		"exprID", res.ExprId)

	if expr, ok := o.expressionByRoot[res.TaskId]; ok && res.TaskId == expr.RootOperationID {
		o.logger.Debug("Found expression by root",
			"res.TaskID", res.TaskId)
		expr.Status = ex.StatusDone.String()
		expr.Success = true
		expr.Result = res.Result

		if err := o.UpdateExpression(ctx, *expr); err != nil {
			o.logger.Error("Failed to update expression after result",
				"error", err)
		}
	}

	o.processPending()
	o.logger.Debug("SubmitResult finished")
	return ack, nil
}

func (o *Orchestrator) ErrorExpression(ctx context.Context, exprID string, exprErr string) error {
	expr, err := o.exprStorage.FindOne(ctx, exprID)
	if err != nil {
		o.logger.Error("Error finding expression in DB",
			"error", err)
		return err
	}

	expr.Error = exprErr
	expr.Status = ex.StatusError.String()
	expr.Success = false

	if err := o.UpdateExpression(ctx, expr); err != nil {
		o.logger.Error("Error updating expression error",
			"error", err)
		return err
	}

	return nil
}

func (o *Orchestrator) UpdateExpression(ctx context.Context, expr ex.Expression) error {
	if err := o.exprStorage.Update(ctx, expr); err != nil {
		o.logger.Error("Error updating expression",
			"error", err)
		return err
	}

	return nil
}
