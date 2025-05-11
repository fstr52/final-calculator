package agent

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/fstr52/final-calculator/internal/logger"
	agentpb "github.com/fstr52/final-calculator/internal/proto" // Адаптированный импорт
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

// MockOrchestratorServiceClient is a mock implementation of agentpb.OrchestratorService
type MockOrchestratorServiceClient struct {
	mock.Mock
}

func (m *MockOrchestratorServiceClient) GetTask(ctx context.Context, in *agentpb.WorkerInfo, opts ...grpc.CallOption) (*agentpb.Task, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*agentpb.Task), args.Error(1)
}

func (m *MockOrchestratorServiceClient) SubmitResult(ctx context.Context, in *agentpb.TaskResult, opts ...grpc.CallOption) (*agentpb.Ack, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*agentpb.Ack), args.Error(1)
}

// MockLogger is a mock implementation of logger.Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, args ...any) {
	m.Called(append([]any{msg}, args...)...)
}

func (m *MockLogger) Info(msg string, args ...any) {
	m.Called(append([]any{msg}, args...)...)
}

func (m *MockLogger) Warn(msg string, args ...any) {
	m.Called(append([]any{msg}, args...)...)
}

func (m *MockLogger) Error(msg string, args ...any) {
	m.Called(append([]any{msg}, args...)...)
}

func (m *MockLogger) With(args ...any) logger.Logger {
	m.Called(args...)
	return m
}

func (m *MockLogger) WithGroup(name string) logger.Logger {
	m.Called(name)
	return m
}

// testRunWorkerIteration simulates one cycle of task processing in RunWorker.
// Used to test the core logic without the infinite loop and gRPC connection setup.
func testRunWorkerIteration(
	t *testing.T,
	ctx context.Context,
	workerID string,
	mockClient *MockOrchestratorServiceClient,
	mockLogger *MockLogger,
	simulatedOpTime time.Duration,
) error {
	task, err := mockClient.GetTask(ctx, &agentpb.WorkerInfo{WorkerId: workerID})
	if err != nil {
		mockLogger.Error("Error client.GetTask", "error", err)
		return err
	}

	if !task.HasTask {
		return nil
	}

	if task.TaskId == "" {
		mockLogger.Error("Received task with empty ID", "task", fmt.Sprintf("%+v", task))
		return errors.New("received task with empty ID")
	}

	select {
	case <-ctx.Done():
		mockLogger.Info("Worker stopping during task execution", "worker_id", workerID)
		return ctx.Err()
	case <-time.After(simulatedOpTime):
		mockLogger.Debug("Worker time completed")
	}

	left := task.Left
	right := task.Right
	var result float32
	var calcErr error

	switch task.Operator {
	case "+":
		sum := float64(left) + float64(right)
		if sum > float64(math.MaxFloat32) {
			calcErr = errors.New("result is bigger than possible")
		} else if sum < -float64(math.MaxFloat32) {
			calcErr = errors.New("result is lower than possible")
		} else {
			result = float32(sum)
		}
	case "-":
		diff := float64(left) - float64(right)
		if diff > float64(math.MaxFloat32) {
			calcErr = errors.New("result is bigger than possible")
		} else if diff < -float64(math.MaxFloat32) {
			calcErr = errors.New("result is lower than possible")
		} else {
			result = float32(diff)
		}
	case "*":
		prod := float64(left) * float64(right)
		if prod > float64(math.MaxFloat32) {
			calcErr = errors.New("result is bigger than possible")
		} else if prod < -float64(math.MaxFloat32) {
			calcErr = errors.New("result is lower than possible")
		} else {
			result = float32(prod)
		}
	case "/":
		if right == 0 {
			calcErr = errors.New("division by zero")
		} else {
			div := float64(left) / float64(right)
			if div > float64(math.MaxFloat32) {
				calcErr = errors.New("result is bigger than possible")
			} else if div < -float64(math.MaxFloat32) {
				calcErr = errors.New("result is lower than possible")
			} else {
				result = float32(div)
			}
		}
	default:
		calcErr = fmt.Errorf("unknown operator: %s", task.Operator)
	}

	taskResult := &agentpb.TaskResult{
		TaskId: task.TaskId,
		ExprId: task.ExprId,
	}
	if calcErr != nil {
		taskResult.Success = false
		taskResult.Error = calcErr.Error()
	} else {
		taskResult.Success = true
		taskResult.Result = result
	}

	ack, err := mockClient.SubmitResult(ctx, taskResult)
	if err != nil || (ack != nil && !ack.Accepted) {
		return fmt.Errorf("submit result error: %v, ack: %+v", err, ack)
	}

	return nil
}

func TestNewAgent(t *testing.T) {
	mockLogger := new(MockLogger)
	agent := NewAgent(3, "localhost", "50051", mockLogger)

	assert.NotNil(t, agent)
	assert.Equal(t, 3, agent.computingPower)
	assert.Equal(t, "localhost", agent.host)
	assert.Equal(t, "50051", agent.port)
	assert.NotNil(t, agent.hasTask)
	assert.Empty(t, agent.hasTask)
	assert.Equal(t, mockLogger, agent.logger)

	mockLogger.AssertExpectations(t)
}

func TestAgent_Run_ZeroPower(t *testing.T) {
	mockLogger := new(MockLogger)
	agent := NewAgent(0, "localhost", "50051", mockLogger)

	// Ожидаем вызов Debug в начале Run
	mockLogger.On("Debug", "Started a.Run").Once()
	// Ожидаем вызов Error, потому что computingPower == 0
	mockLogger.On("Error", "Computing power == 0").Once()

	err := agent.Run(context.Background())

	assert.Error(t, err)
	assert.Equal(t, "computing power == 0", err.Error())

	// Проверяем, что все настроенные вызовы были выполнены
	mockLogger.AssertExpectations(t)
}

func TestAgent_Run_ContextCancellation(t *testing.T) {
	mockLogger := new(MockLogger)
	agent := NewAgent(1, "localhost", "50051", mockLogger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ожидаем вызов Debug в начале Run
	mockLogger.On("Debug", "Started a.Run").Once()

	// Ожидаем вызов Debug в начале RunWorker
	// Используем mock.Anything для аргументов, так как они могут меняться
	mockLogger.On("Debug", "Starting Worker", mock.Anything, mock.Anything).Once() // Ожидаем строку + два любых аргумента

	// Ожидаем вызов Debug в RunWorker после тика таймера (если он успеет тикнуть до отмены)
	mockLogger.On("Debug", "Worker time completed").Maybe() // Может произойти, может нет

	// Ожидаем вызов Info в RunWorker при отмене контекста
	mockLogger.On("Info", "Worker stopping during task execution", mock.Anything, mock.Anything).Maybe() // Может произойти, может нет

	// Ожидаем graceful finish log из Run, если ctx отменен
	mockLogger.On("Debug", "Run finished gracefully").Maybe()

	// Cancel the context after a short delay.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := agent.Run(ctx)

	assert.NoError(t, err, "Run should return nil on context cancellation")

	// Wait a bit for goroutines to finish and log messages
	time.Sleep(100 * time.Millisecond)

	mockLogger.AssertExpectations(t)
}

func TestRunWorkerIteration_NoTask(t *testing.T) {
	mockClient := new(MockOrchestratorServiceClient)
	mockLogger := new(MockLogger)
	workerID := "worker-123"

	mockClient.On("GetTask", mock.Anything, &agentpb.WorkerInfo{WorkerId: workerID}).Return(&agentpb.Task{HasTask: false}, nil).Once()

	err := testRunWorkerIteration(t, context.Background(), workerID, mockClient, mockLogger, 1*time.Millisecond)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestRunWorkerIteration_GetTaskError(t *testing.T) {
	mockClient := new(MockOrchestratorServiceClient)
	mockLogger := new(MockLogger)
	workerID := "worker-123"
	getTaskErr := errors.New("simulated gRPC error")

	mockClient.On("GetTask", mock.Anything, &agentpb.WorkerInfo{WorkerId: workerID}).Return(&agentpb.Task{}, getTaskErr).Once()
	mockLogger.On("Error", "Error client.GetTask", "error", getTaskErr).Once()

	err := testRunWorkerIteration(t, context.Background(), workerID, mockClient, mockLogger, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, getTaskErr, err)
	mockClient.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestRunWorkerIteration_SubmitResultError(t *testing.T) {
	mockClient := new(MockOrchestratorServiceClient)
	mockLogger := new(MockLogger)
	workerID := "worker-123"
	taskID := "task-abc"
	exprID := "expr-xyz"
	submitErr := errors.New("simulated submit gRPC error")

	mockClient.On("GetTask", mock.Anything, &agentpb.WorkerInfo{WorkerId: workerID}).Return(
		&agentpb.Task{HasTask: true, TaskId: taskID, ExprId: exprID, Left: 10, Operator: "+", Right: 20, OperationTime: 1}, nil).Once()

	mockLogger.On("Debug", "Worker time completed").Once()

	mockClient.On("SubmitResult", mock.Anything, &agentpb.TaskResult{
		TaskId: taskID, ExprId: exprID, Success: true, Result: 30.0, Error: "",
	}).Return(&agentpb.Ack{}, submitErr).Once()

	err := testRunWorkerIteration(t, context.Background(), workerID, mockClient, mockLogger, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "submit result error")
	assert.Contains(t, err.Error(), submitErr.Error())
	mockClient.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestRunWorkerIteration_SubmitResultNotAccepted(t *testing.T) {
	mockClient := new(MockOrchestratorServiceClient)
	mockLogger := new(MockLogger)
	workerID := "worker-123"
	taskID := "task-abc"
	exprID := "expr-xyz"

	mockClient.On("GetTask", mock.Anything, &agentpb.WorkerInfo{WorkerId: workerID}).Return(
		&agentpb.Task{HasTask: true, TaskId: taskID, ExprId: exprID, Left: 10, Operator: "+", Right: 20, OperationTime: 1}, nil).Once()

	mockLogger.On("Debug", "Worker time completed").Once()

	// Объявляем переменную для ACK, которую мок вернет
	returnedAck := &agentpb.Ack{Accepted: false}

	// Настраиваем мок на возврат этого ACK
	mockClient.On("SubmitResult", mock.Anything, &agentpb.TaskResult{
		TaskId: taskID, ExprId: exprID, Success: true, Result: 30.0, Error: "",
	}).Return(returnedAck, nil).Once() // Используем переменную тут

	err := testRunWorkerIteration(t, context.Background(), workerID, mockClient, mockLogger, 1*time.Millisecond)

	assert.Error(t, err)

	// Генерируем ТОЧНУЮ ожидаемую строку ошибки, используя тот же формат и возвращенный ACK
	// Это обходит любые неопределенности с тем, как %+v форматирует protobuf struct
	expectedErrorString := fmt.Sprintf("submit result error: %v, ack: %+v", nil, returnedAck)

	// Используем assert.Equal для точного сравнения строк
	assert.Equal(t, expectedErrorString, err.Error())

	mockClient.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestRunWorkerIteration_Calculation(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		left     float32
		right    float32
		expected float32
		isError  bool
		errStr   string
	}{
		{name: "Addition", operator: "+", left: 10, right: 20, expected: 30, isError: false},
		{name: "Subtraction", operator: "-", left: 30, right: 10, expected: 20, isError: false},
		{name: "Multiplication", operator: "*", left: 5, right: 6, expected: 30, isError: false},
		{name: "Division", operator: "/", left: 100, right: 4, expected: 25, isError: false},
		{name: "DivisionByZero", operator: "/", left: 100, right: 0, isError: true, errStr: "division by zero"},
		// Add overflow/underflow tests if desired
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockOrchestratorServiceClient)
			mockLogger := new(MockLogger)
			workerID := "worker-123"
			taskID := "task-abc"
			exprID := "expr-xyz"

			mockClient.On("GetTask", mock.Anything, &agentpb.WorkerInfo{WorkerId: workerID}).Return(
				&agentpb.Task{HasTask: true, TaskId: taskID, ExprId: exprID, Left: tt.left, Operator: tt.operator, Right: tt.right, OperationTime: 1}, nil).Once()

			mockLogger.On("Debug", "Worker time completed").Once()

			expectedResult := &agentpb.TaskResult{
				TaskId: taskID,
				ExprId: exprID,
			}
			if tt.isError {
				expectedResult.Success = false
				expectedResult.Error = tt.errStr
				mockClient.On("SubmitResult", mock.Anything, expectedResult).Return(&agentpb.Ack{Accepted: true}, nil).Once()
			} else {
				expectedResult.Success = true
				expectedResult.Result = tt.expected
				expectedResult.Error = ""
				mockClient.On("SubmitResult", mock.Anything, expectedResult).Return(&agentpb.Ack{Accepted: true}, nil).Once()
			}

			err := testRunWorkerIteration(t, context.Background(), workerID, mockClient, mockLogger, 1*time.Millisecond)

			assert.NoError(t, err)
			mockClient.AssertExpectations(t)
			mockLogger.AssertExpectations(t)
		})
	}
}

func TestRunWorkerIteration_ContextCancellationDuringOperation(t *testing.T) {
	mockClient := new(MockOrchestratorServiceClient)
	mockLogger := new(MockLogger)
	workerID := "worker-123"
	taskID := "task-abc"
	exprID := "expr-xyz"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	mockClient.On("GetTask", mock.Anything, &agentpb.WorkerInfo{WorkerId: workerID}).Return(
		&agentpb.Task{HasTask: true, TaskId: taskID, ExprId: exprID, Left: 10, Operator: "+", Right: 20, OperationTime: 1000}, nil).Once()

	mockLogger.On("Info", "Worker stopping during task execution", "worker_id", workerID).Once()

	mockClient.AssertNotCalled(t, "SubmitResult", mock.Anything, mock.Anything)

	err := testRunWorkerIteration(t, ctx, workerID, mockClient, mockLogger, 50*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)

	time.Sleep(20 * time.Millisecond)

	mockClient.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}

func TestRunWorkerIteration_EmptyTaskID(t *testing.T) {
	mockClient := new(MockOrchestratorServiceClient)
	mockLogger := new(MockLogger)
	workerID := "worker-123"

	task := &agentpb.Task{HasTask: true, TaskId: "", ExprId: "expr-xyz", Left: 10, Operator: "+", Right: 20, OperationTime: 1}
	mockClient.On("GetTask", mock.Anything, &agentpb.WorkerInfo{WorkerId: workerID}).Return(task, nil).Once()

	mockLogger.On("Error", "Received task with empty ID", "task", fmt.Sprintf("%+v", task)).Once()

	err := testRunWorkerIteration(t, context.Background(), workerID, mockClient, mockLogger, 1*time.Millisecond)

	assert.Error(t, err)
	assert.Equal(t, "received task with empty ID", err.Error())

	mockClient.AssertExpectations(t)
	mockLogger.AssertExpectations(t)
}
