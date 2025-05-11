package agent

import (
	"context"
	"errors"
	"fmt"
	"math"
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

			if !task.HasTask {
				continue
			}

			if task.TaskId == "" {
				a.logger.Error("Received task with empty ID",
					"task", fmt.Sprintf("%+v", task))
			}

			left := task.Left
			right := task.Right

			var result float32

			timer := time.NewTimer(time.Duration(task.OperationTime * time.Hour.Milliseconds()))

			switch task.Operator {
			case "+":
				sum := float64(left) + float64(right)
				if sum > float64(math.MaxFloat32) {
					err = errors.New("result is bigger than possible")
				} else if sum < -float64(math.MaxFloat32) {
					err = errors.New("result is lower than possible")
				} else {
					result = float32(sum)
				}
			case "-":
				diff := float64(left) - float64(right)
				if diff > float64(math.MaxFloat32) {
					err = errors.New("result is bigger than possible")
				} else if diff < -float64(math.MaxFloat32) {
					err = errors.New("result is lower than possible")
				} else {
					result = float32(diff)
				}
			case "*":
				prod := float64(left) * float64(right)
				if prod > float64(math.MaxFloat32) {
					err = errors.New("result is bigger than possible")
				} else if prod < -float64(math.MaxFloat32) {
					err = errors.New("result is lower than possible")
				} else {
					result = float32(prod)
				}
			case "/":
				if right == 0 {
					err = errors.New("division by zero")
				} else {
					div := float64(left) / float64(right)
					if div > float64(math.MaxFloat32) {
						err = errors.New("result is bigger than possible")
					} else if div < -float64(math.MaxFloat32) {
						err = errors.New("result is lower than possible")
					} else {
						result = float32(div)
					}
				}
			}

			select {
			case <-ctx.Done():
				a.logger.Info("Worker stopping during task execution", "worker_id", id)
				return nil
			case <-timer.C:
				a.logger.Debug("Worker time completed")
			}

			if err != nil {
				taskResult := &pr.TaskResult{
					TaskId:  task.TaskId,
					Success: false,
					Error:   err.Error(),
					ExprId:  task.ExprId,
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
					ExprId:  task.ExprId,
				}

				ack, err := client.SubmitResult(ctx, taskResult)
				if err != nil || !ack.Accepted {
					return fmt.Errorf("submit result error: %v, ack: %+v", err, ack)
				}
			}
		}
	}
}
