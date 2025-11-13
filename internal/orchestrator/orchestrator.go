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

type OperationsConfig struct {
	TimeAdditionMS        int64
	TimeSubtractionMS     int64
	TimeMultiplicationsMS int64
	TimeDivisionsMS       int64
}

type Orchestrator struct {
	pr.OrchestratorServiceServer
	logger logger.Logger
	oc     OperationsConfig

	expressionQueue  []*ex.Expression
	doneCache        map[string]float64
	opErrorCache     map[string]string
	pendingOps       map[string]op.Operation
	expressionByRoot map[string]*ex.Expression
	toSend           []op.Operation
	exprStorage      ex.Storage
	opStorage        op.Storage

	pendingMu sync.Mutex
	cacheMu   sync.RWMutex
	exprMu    sync.Mutex
	sendMu    sync.Mutex
}

func NewOrchestrator(logger logger.Logger, client postgresql.Client, oc OperationsConfig) *Orchestrator {
	return &Orchestrator{
		logger:           logger,
		expressionQueue:  make([]*ex.Expression, 0),
		doneCache:        make(map[string]float64),
		opErrorCache:     make(map[string]string),
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

		for _, oper := range ops {
			if oper.Status == "done" {
				o.cacheMu.Lock()
				o.doneCache[oper.ID] = oper.Result
				o.cacheMu.Unlock()
				continue
			}
			if oper.Status == op.StatusError.String() || oper.Status == "error" {
				o.cacheMu.Lock()
				o.opErrorCache[oper.ID] = oper.Error
				o.cacheMu.Unlock()
				continue
			}

			ready := true
			failed := false
			o.cacheMu.RLock()
			for _, dep := range oper.Dependencies {
				if _, ok := o.doneCache[dep]; !ok {
					if _, fail := o.opErrorCache[dep]; fail {
						failed = true
						break
					}
					ready = false
					break
				}
			}
			o.cacheMu.RUnlock()

			if failed {
				_ = o.SetOperationError(ctx, oper.ID, "dependency failed during restore")
				continue
			}

			if ready {
				o.enqueue(oper)
			} else {
				o.pendingMu.Lock()
				o.pendingOps[oper.ID] = oper
				o.pendingMu.Unlock()
			}
		}
	}

	o.logger.Info("State restored successfully")
}

func (o *Orchestrator) NewExpression(ctx context.Context, input string, userID string) (*ex.Expression, error) {
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
		Success:    false,
	}

	if err = o.exprStorage.Create(ctx, *expr); err != nil {
		return nil, err
	}

	rootId := o.planOperations(ctx, expr, ast)
	expr.RootOperationID = rootId

	if _, ok := ast.(*parser.Number); ok {
		o.cacheMu.RLock()
		res := o.doneCache[rootId]
		o.cacheMu.RUnlock()

		expr.Status = ex.StatusDone.String()
		expr.Success = true
		expr.Result = res

		if err := o.UpdateExpression(ctx, *expr); err != nil {
			return nil, err
		}

		o.expressionByRoot[rootId] = expr
		return expr, nil
	}

	o.exprMu.Lock()
	o.expressionQueue = append(o.expressionQueue, expr)
	o.exprMu.Unlock()
	expr.Status = ex.StatusInQueue.String()
	o.expressionByRoot[rootId] = expr

	if err := o.UpdateExpression(ctx, *expr); err != nil {
		return nil, err
	}

	o.logger.Debug("Successfully created new expression",
		"Expression", expr.String())
	return expr, nil
}

func (o *Orchestrator) planOperations(ctx context.Context, expr *ex.Expression, ast parser.Expr) string {
	switch n := ast.(type) {
	case *parser.Number:
		id := uuid.NewString()
		o.cacheMu.Lock()
		o.doneCache[id] = ast.Eval()
		o.cacheMu.Unlock()
		return id
	case *parser.Binary:
		leftId := o.planOperations(ctx, expr, n.Left)
		rightId := o.planOperations(ctx, expr, n.Right)

		if leftId == "" || rightId == "" {
			o.logger.Error("planOperations got empty left/rightId",
				"leftId", leftId,
				"rightId", rightId,
				"op", n.Symbol.Value)
			return ""
		}

		var opTime int64
		switch n.Symbol.Value {
		case "+":
			opTime = o.oc.TimeAdditionMS
		case "-":
			opTime = o.oc.TimeSubtractionMS
		case "*":
			opTime = o.oc.TimeMultiplicationsMS
		case "/":
			opTime = o.oc.TimeDivisionsMS
		}

		id := uuid.NewString()
		oper := op.Operation{
			ID:            id,
			Left:          leftId,
			Operator:      n.Symbol.Value,
			Right:         rightId,
			Dependencies:  []string{leftId, rightId},
			ExprID:        expr.ID,
			OperationTime: opTime,
		}

		oper.Status = op.StatusPending.String()
		if err := o.opStorage.Create(ctx, oper); err != nil {
			o.logger.Error("Failed to save operation", "error", err)
		}

		expr.Operations = append(expr.Operations, oper)
		return id

	default:
		o.logger.Error("planOperations: unknown AST node type",
			"type", fmt.Sprintf("%T", ast))
		o.logger.Error("unknown AST node type")
		return ""
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
			o.nextExpr(ctx)
			o.processPending(ctx)
		}
	}
}

func (o *Orchestrator) nextExpr(ctx context.Context) {
	if len(o.expressionQueue) == 0 {
		return
	}

	o.exprMu.Lock()
	expr := o.expressionQueue[0]
	o.expressionQueue = o.expressionQueue[1:]
	o.exprMu.Unlock()

	for _, op := range expr.Operations {
		ready := true
		failed := false

		for _, dep := range op.Dependencies {
			o.cacheMu.RLock()
			_, done := o.doneCache[dep]
			_, errorDep := o.opErrorCache[dep]
			o.cacheMu.RUnlock()

			if !done {
				if errorDep {
					errMsg := fmt.Sprintf("dependency %s failed", dep)
					_ = o.SetOperationError(ctx, op.ID, errMsg)
					failed = true
					break
				}
				ready = false
				break
			}
		}

		if failed {
			continue
		}

		if ready {
			o.enqueue(op)
		} else {
			o.pendingMu.Lock()
			o.pendingOps[op.ID] = op
			o.pendingMu.Unlock()
		}
	}
}

func (o *Orchestrator) processPending(ctx context.Context) {
	o.pendingMu.Lock()
	defer o.pendingMu.Unlock()

	if len(o.pendingOps) == 0 {
		return
	}

	var toDelete []string

	for id, op := range o.pendingOps {
		allReady := true
		failed := false

		for _, dep := range op.Dependencies {
			o.cacheMu.RLock()
			_, done := o.doneCache[dep]
			_, errorDep := o.opErrorCache[dep]
			o.cacheMu.RUnlock()

			if !done {
				if errorDep {
					errMsg := fmt.Sprintf("dependency %s failed", dep)
					_ = o.SetOperationError(ctx, id, errMsg)
					toDelete = append(toDelete, id)
					failed = true
					break
				}
				allReady = false
				break
			}
		}
		if failed {
			continue
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
		if err := o.SetOperationError(ctx, res.TaskId, res.Error); err != nil {
			o.logger.Error("Failed to set op error", "opID", res.TaskId, "error", err)
		}

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

	o.processPending(ctx)
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

func (o *Orchestrator) SetOperationError(ctx context.Context, opID string, errMsg string) error {
	oper, err := o.opStorage.FindOne(ctx, opID)
	if err != nil {
		o.logger.Error("Failed to find operation", "opID", opID, "error", err)
		return err
	}
	oper.Status = op.StatusError.String()
	oper.Error = errMsg

	o.cacheMu.Lock()
	o.opErrorCache[opID] = errMsg
	o.cacheMu.Unlock()

	return o.opStorage.Update(ctx, oper)
}
