package agent

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/fstr52/final-calculator/internal/logger"
	pr "github.com/fstr52/final-calculator/internal/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Agent struct {
	computingPower int
	hasTask        map[string]bool
	mu             sync.Mutex
	host           string
	port           string
	logger         logger.Logger
}

type OperationRequest struct {
	OperationID  string
	Left         string
	Operator     string
	Right        string
	Dependencies map[string]bool
}

func NewAgent(computingPower int, host string, port string, logger logger.Logger) *Agent {
	return &Agent{
		computingPower: computingPower,
		host:           host,
		port:           port,
		hasTask:        make(map[string]bool),
		logger:         logger,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	a.logger.Debug("Started a.Run")
	if a.computingPower == 0 {
		a.logger.Error("Computing power == 0")
		return fmt.Errorf("computing power == 0")
	}

	errChan := make(chan error, 1)

	for range a.computingPower {
		workerId := uuid.NewString()
		a.mu.Lock()
		a.hasTask[workerId] = false
		a.mu.Unlock()

		go func() {
			err := a.RunWorker(ctx, workerId, a.host, a.port)
			if err != nil {
				a.logger.Error("Worker error",
					"error", err)
				errChan <- err
			}
		}()
	}

	select {
	case <-ctx.Done():
		a.logger.Debug("Run finished gracefully")
		return nil
	case err := <-errChan:
		return err
	}
}

func (a *Agent) RunWorker(ctx context.Context, id string, host string, port string) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	addr := fmt.Sprintf("%s:%s", host, port)
	a.logger.Debug("Starting Worker",
		"orchestratorAddr", addr)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		a.logger.Error("Error starting grpc.NewClient",
			"error", err)
		return err
	}
	defer conn.Close()

	client := pr.NewOrchestratorServiceClient(conn)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			task, err := client.GetTask(ctx, &pr.WorkerInfo{WorkerId: id})
			if err != nil {
				a.logger.Error("Error client.GetTask",
					"error", err)
				return err
			}

			left := task.Left
			right := task.Right

			var result float32
			switch task.Operator {
			case "+":
				result = left + right
			case "-":
				result = left - right
			case "*":
				result = left * right
			case "/":
				if right == 0 {
					err = errors.New("division by zero")
				} else {
					result = left / right
				}
			}

			if err != nil {
				taskResult := &pr.TaskResult{
					TaskId:  task.TaskId,
					Success: false,
					Error:   err.Error(),
				}

				ack, err := client.SubmitResult(ctx, taskResult)
				if err != nil || !ack.Accepted {
					return fmt.Errorf("submit result error: %v, ack: %+v", err, ack)
				}
			} else {
				taskResult := &pr.TaskResult{
					TaskId:  task.TaskId,
					Success: true,
					Result:  result,
				}

				ack, err := client.SubmitResult(ctx, taskResult)
				if err != nil || !ack.Accepted {
					return fmt.Errorf("submit result error: %v, ack: %+v", err, ack)
				}
			}
		}
	}
}
