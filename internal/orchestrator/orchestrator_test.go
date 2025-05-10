package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/fstr52/final-calculator/internal/logger"
	pr "github.com/fstr52/final-calculator/internal/proto"
	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockClient имитирует клиент postgresql
type MockClient struct {
	mock.Mock
}

func (m *MockClient) Begin(ctx context.Context) (pgx.Tx, error) {
	args := m.Called(ctx)
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func (m *MockClient) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *MockClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	mockArgs := m.Called(ctx, sql, args)
	return mockArgs.Get(0).(pgx.Rows), mockArgs.Error(1)
}

func (m *MockClient) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	mockArgs := m.Called(ctx, sql, args)
	return mockArgs.Get(0).(pgx.Row)
}

// MockLogger имитирует интерфейс логгера
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, args ...any) {
	m.Called(msg, args)
}

func (m *MockLogger) Info(msg string, args ...any) {
	m.Called(msg, args)
}

func (m *MockLogger) Warn(msg string, args ...any) {
	m.Called(msg, args)
}

func (m *MockLogger) Error(msg string, args ...any) {
	m.Called(msg, args)
}

func (m *MockLogger) With(args ...any) logger.Logger {
	mockArgs := m.Called(args)
	return mockArgs.Get(0).(logger.Logger)
}

func (m *MockLogger) WithGroup(name string) logger.Logger {
	args := m.Called(name)
	return args.Get(0).(logger.Logger)
}

func setupMockLogger() *MockLogger {
	mockLogger := new(MockLogger)
	mockLogger.On("Debug", mock.Anything, mock.Anything).Return()
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()
	mockLogger.On("Warn", mock.Anything, mock.Anything).Return()
	mockLogger.On("Error", mock.Anything, mock.Anything).Return()
	mockLogger.On("With", mock.Anything).Return(mockLogger)
	mockLogger.On("WithGroup", mock.Anything).Return(mockLogger)
	return mockLogger
}

func TestNewOrchestrator(t *testing.T) {
	mockLogger := setupMockLogger()
	mockClient := new(MockClient)

	orch := NewOrchestrator(mockLogger, mockClient)

	assert.NotNil(t, orch)
	assert.Equal(t, mockLogger, orch.logger)
	assert.Empty(t, orch.expressionQueue)
	assert.Empty(t, orch.doneCache)
	assert.Empty(t, orch.pendingOps)
	assert.NotNil(t, orch.exprStorage)
}

func TestNewExpression(t *testing.T) {
	mockLogger := setupMockLogger()
	mockClient := new(MockClient)

	orch := NewOrchestrator(mockLogger, mockClient)

	// Тест валидного выражения
	expr, err := orch.NewExpression("2+3*4")
	assert.NoError(t, err)
	assert.NotNil(t, expr)
	assert.Equal(t, StatusInQueue, expr.Status)
	assert.NotEmpty(t, expr.Id)
	assert.NotEmpty(t, expr.Operations)

	// Тест невалидного выражения
	expr, err = orch.NewExpression("2++")
	assert.Error(t, err)
	assert.Nil(t, expr)
}

func TestGetTask(t *testing.T) {
	mockLogger := setupMockLogger()
	mockClient := new(MockClient)

	orch := NewOrchestrator(mockLogger, mockClient)
	ctx := context.Background()

	// Тест на пустую очередь
	task, err := orch.GetTask(ctx, &pr.WorkerInfo{WorkerId: "worker1"})
	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.False(t, task.HasTask)

	// Добавляем операцию в очередь
	opID := uuid.NewString()
	leftID := uuid.NewString()
	rightID := uuid.NewString()
	exprID := uuid.NewString()

	orch.cacheMu.Lock()
	orch.doneCache[leftID] = 10.0
	orch.doneCache[rightID] = 20.0
	orch.cacheMu.Unlock()

	op := OperationRequest{
		Id:           opID,
		Left:         leftID,
		Operator:     "+",
		Right:        rightID,
		HasTask:      true,
		Dependencies: []string{leftID, rightID},
		ExprId:       exprID,
	}

	orch.sendMu.Lock()
	orch.toSend = append(orch.toSend, op)
	orch.sendMu.Unlock()

	// Получаем задачу
	task, err = orch.GetTask(ctx, &pr.WorkerInfo{WorkerId: "worker1"})
	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.True(t, task.HasTask)
	assert.Equal(t, opID, task.TaskId)
	assert.Equal(t, float32(10.0), task.Left)
	assert.Equal(t, "+", task.Operator)
	assert.Equal(t, float32(20.0), task.Right)
}

func TestSubmitResult(t *testing.T) {
	mockLogger := setupMockLogger()
	mockClient := new(MockClient)

	orch := NewOrchestrator(mockLogger, mockClient)
	ctx := context.Background()

	// Подготовка данных
	taskID := uuid.NewString()
	exprID := uuid.NewString()

	orch.cacheMu.Lock()
	orch.doneCache[taskID] = 0 // Временное значение
	orch.cacheMu.Unlock()

	// Тест успешного результата
	result := &pr.TaskResult{
		TaskId: taskID,
		Result: 30.0,
		ExprId: exprID,
	}

	ack, err := orch.SubmitResult(ctx, result)
	assert.NoError(t, err)
	assert.NotNil(t, ack)
	assert.True(t, ack.Accepted)

	// Проверка, что результат сохранен
	orch.cacheMu.RLock()
	val, exists := orch.doneCache[taskID]
	orch.cacheMu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, float32(30.0), val)

	// Тест с ошибкой в результате
	errorTaskID := uuid.NewString()
	orch.cacheMu.Lock()
	orch.doneCache[errorTaskID] = 0
	orch.cacheMu.Unlock()

	errorResult := &pr.TaskResult{
		TaskId: errorTaskID,
		Error:  "division by zero",
		ExprId: exprID,
	}

	ack, err = orch.SubmitResult(ctx, errorResult)
	assert.NoError(t, err)
	assert.NotNil(t, ack)
	assert.True(t, ack.Accepted)
}

func TestProcessingWorkflow(t *testing.T) {
	mockLogger := setupMockLogger()
	mockClient := new(MockClient)

	orch := NewOrchestrator(mockLogger, mockClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем оркестратор
	orch.Run(ctx)

	// Создаем и добавляем выражение
	exprID := uuid.NewString()
	leftID := uuid.NewString()
	rightID := uuid.NewString()
	opID := uuid.NewString()

	// Добавляем результаты зависимостей
	orch.cacheMu.Lock()
	orch.doneCache[leftID] = 10.0
	orch.doneCache[rightID] = 20.0
	// Важно: добавляем сам opID в doneCache с временным значением
	orch.doneCache[opID] = 0.0
	orch.cacheMu.Unlock()

	// Создаем операцию и добавляем ее в pendingOps
	op := OperationRequest{
		Id:           opID,
		Left:         leftID,
		Operator:     "+",
		Right:        rightID,
		Dependencies: []string{leftID, rightID},
		ExprId:       exprID,
	}

	orch.pendingOps[opID] = op

	// Тестируем processPending
	orch.processPending()

	// Проверяем, что операция перемещена из pendingOps в toSend
	assert.NotContains(t, orch.pendingOps, opID)

	orch.sendMu.Lock()
	found := false
	for _, sendOp := range orch.toSend {
		if sendOp.Id == opID {
			found = true
			break
		}
	}
	orch.sendMu.Unlock()

	assert.True(t, found, "Операция должна быть добавлена в toSend")

	// Тестируем получение задачи
	worker := &pr.WorkerInfo{WorkerId: "test-worker"}
	task, err := orch.GetTask(ctx, worker)
	assert.NoError(t, err)
	assert.True(t, task.HasTask)
	assert.Equal(t, opID, task.TaskId)
	assert.Equal(t, "+", task.Operator)
	assert.Equal(t, float32(10.0), task.Left)
	assert.Equal(t, float32(20.0), task.Right)

	// Тестируем отправку результата
	result := &pr.TaskResult{
		TaskId: opID,
		Result: 30.0,
		ExprId: exprID,
	}

	ack, err := orch.SubmitResult(ctx, result)
	assert.NoError(t, err)
	assert.True(t, ack.Accepted)

	// Проверяем, что результат сохранен в кэше
	orch.cacheMu.RLock()
	val, exists := orch.doneCache[opID]
	orch.cacheMu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, float32(30.0), val)
}

func TestEnqueue(t *testing.T) {
	mockLogger := setupMockLogger()
	mockClient := new(MockClient)

	orch := NewOrchestrator(mockLogger, mockClient)

	// Создаем операцию
	opID := uuid.NewString()
	leftID := uuid.NewString()
	rightID := uuid.NewString()
	exprID := uuid.NewString()

	op := OperationRequest{
		Id:           opID,
		Left:         leftID,
		Operator:     "*",
		Right:        rightID,
		Dependencies: []string{leftID, rightID},
		ExprId:       exprID,
	}

	// Проверяем, что очередь пуста
	assert.Empty(t, orch.toSend)

	// Добавляем операцию в очередь
	orch.enqueue(op)

	// Проверяем, что операция добавлена
	assert.Equal(t, 1, len(orch.toSend))
	assert.Equal(t, opID, orch.toSend[0].Id)
}

func TestNextExpr(t *testing.T) {
	mockLogger := setupMockLogger()
	mockClient := new(MockClient)

	orch := NewOrchestrator(mockLogger, mockClient)

	// Создаем выражение с операциями
	exprID := uuid.NewString()
	leftID := uuid.NewString()
	rightID := uuid.NewString()
	opID := uuid.NewString()

	expr := &Expression{
		Id:     exprID,
		Status: StatusInQueue,
		Operations: []OperationRequest{
			{
				Id:           opID,
				Left:         leftID,
				Operator:     "+",
				Right:        rightID,
				Dependencies: []string{leftID, rightID},
				ExprId:       exprID,
			},
		},
	}

	// Добавляем выражение в очередь
	orch.exprMu.Lock()
	orch.expressionQueue = append(orch.expressionQueue, expr)
	orch.exprMu.Unlock()

	// Проверяем работу nextExpr с незавершенными зависимостями
	orch.nextExpr()
	assert.Empty(t, orch.expressionQueue)
	assert.Contains(t, orch.pendingOps, opID)

	// Добавляем результаты зависимостей
	orch.cacheMu.Lock()
	orch.doneCache[leftID] = 10.0
	orch.doneCache[rightID] = 20.0
	orch.cacheMu.Unlock()

	// Добавляем еще одно выражение
	expr2ID := uuid.NewString()
	op2ID := uuid.NewString()

	expr2 := &Expression{
		Id:     expr2ID,
		Status: StatusInQueue,
		Operations: []OperationRequest{
			{
				Id:           op2ID,
				Left:         leftID, // Используем те же зависимости, что уже выполнены
				Operator:     "-",
				Right:        rightID,
				Dependencies: []string{leftID, rightID},
				ExprId:       expr2ID,
			},
		},
	}

	// Добавляем второе выражение в очередь
	orch.exprMu.Lock()
	orch.expressionQueue = append(orch.expressionQueue, expr2)
	orch.exprMu.Unlock()

	// Проверяем, что операция с выполненными зависимостями добавляется в toSend
	orch.nextExpr()
	assert.Empty(t, orch.expressionQueue)

	orch.sendMu.Lock()
	foundInToSend := false
	for _, op := range orch.toSend {
		if op.Id == op2ID {
			foundInToSend = true
			break
		}
	}
	orch.sendMu.Unlock()

	assert.True(t, foundInToSend, "Операция должна быть добавлена в toSend")
}

func TestRun(t *testing.T) {
	mockLogger := setupMockLogger()
	mockClient := new(MockClient)

	orch := NewOrchestrator(mockLogger, mockClient)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем оркестратор
	orch.Run(ctx)

	// Даем время на запуск горутины
	time.Sleep(200 * time.Millisecond)

	// Создаем операцию, которую можно сразу выполнить
	opID := uuid.NewString()
	leftID := uuid.NewString()
	rightID := uuid.NewString()
	exprID := uuid.NewString()

	// Добавляем зависимости в кэш
	orch.cacheMu.Lock()
	orch.doneCache[leftID] = 15.0
	orch.doneCache[rightID] = 5.0
	orch.cacheMu.Unlock()

	// Добавляем операцию в pendingOps
	op := OperationRequest{
		Id:           opID,
		Left:         leftID,
		Operator:     "*",
		Right:        rightID,
		Dependencies: []string{leftID, rightID},
		ExprId:       exprID,
	}

	orch.pendingOps[opID] = op

	// Даем время планировщику обработать операцию
	time.Sleep(300 * time.Millisecond)

	// Проверяем, что операция была перемещена в toSend
	orch.sendMu.Lock()
	foundInToSend := false
	for _, sendOp := range orch.toSend {
		if sendOp.Id == opID {
			foundInToSend = true
			break
		}
	}
	orch.sendMu.Unlock()

	assert.True(t, foundInToSend, "Операция должна быть обработана планировщиком и добавлена в toSend")
	assert.NotContains(t, orch.pendingOps, opID, "Операция должна быть удалена из pendingOps")
}
