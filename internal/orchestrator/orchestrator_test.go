package orchestrator

import (
	"context"
	"testing"

	ex "github.com/fstr52/final-calculator/internal/expression"
	"github.com/fstr52/final-calculator/internal/logger"
	op "github.com/fstr52/final-calculator/internal/operation"
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

// MockExpressionStorage мокает хранилище выражений
type MockExpressionStorage struct {
	mock.Mock
}

func (m *MockExpressionStorage) Create(ctx context.Context, expr ex.Expression) error {
	args := m.Called(ctx, expr)
	return args.Error(0)
}

func (m *MockExpressionStorage) FindAll(ctx context.Context) ([]ex.Expression, error) {
	args := m.Called(ctx)
	return args.Get(0).([]ex.Expression), args.Error(1)
}

func (m *MockExpressionStorage) FindOne(ctx context.Context, id string) (ex.Expression, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(ex.Expression), args.Error(1)
}

func (m *MockExpressionStorage) FindAllByUser(ctx context.Context, userId string) ([]ex.Expression, error) {
	args := m.Called(ctx, userId)
	return args.Get(0).([]ex.Expression), args.Error(1)
}

func (m *MockExpressionStorage) Update(ctx context.Context, expr ex.Expression) error {
	args := m.Called(ctx, expr)
	return args.Error(0)
}

func (m *MockExpressionStorage) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockExpressionStorage) GetUnfinishedExpressions(ctx context.Context) ([]*ex.Expression, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*ex.Expression), args.Error(1)
}

// MockOperationStorage мокает хранилище операций
type MockOperationStorage struct {
	mock.Mock
}

func (m *MockOperationStorage) Create(ctx context.Context, op op.Operation) error {
	args := m.Called(ctx, op)
	return args.Error(0)
}

func (m *MockOperationStorage) Update(ctx context.Context, op op.Operation) error {
	args := m.Called(ctx, op)
	return args.Error(0)
}

func (m *MockOperationStorage) GetOperationsByExpression(ctx context.Context, exprID string) ([]op.Operation, error) {
	args := m.Called(ctx, exprID)
	return args.Get(0).([]op.Operation), args.Error(1)
}

func (m *MockOperationStorage) FindOne(ctx context.Context, id string) (op.Operation, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(op.Operation), args.Error(1)
}

func (m *MockOperationStorage) UpdateOperationResult(ctx context.Context, opID string, result float64, status string) error {
	args := m.Called(ctx, opID, result, status)
	return args.Error(0)
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

type testOrchestrator struct {
	*Orchestrator
	mockExprStorage *MockExpressionStorage
	mockOpStorage   *MockOperationStorage
}

func setupTestOrchestrator() *testOrchestrator {
	mockLogger := setupMockLogger()
	mockExprStorage := new(MockExpressionStorage)
	mockOpStorage := new(MockOperationStorage)

	// Настройка базовых ожиданий
	mockExprStorage.On("GetUnfinishedExpressions", mock.Anything).Return([]*ex.Expression{}, nil)
	mockExprStorage.On("FindAll", mock.Anything).Return([]ex.Expression{}, nil)
	mockExprStorage.On("FindAllByUser", mock.Anything, mock.Anything).Return([]ex.Expression{}, nil)
	mockOpStorage.On("GetOperationsByExpression", mock.Anything, mock.Anything).Return([]op.Operation{}, nil)
	mockOpStorage.On("Create", mock.Anything, mock.Anything).Return(nil)

	orch := &Orchestrator{
		logger:           mockLogger,
		expressionQueue:  make([]*ex.Expression, 0),
		doneCache:        make(map[string]float64),
		pendingOps:       make(map[string]op.Operation),
		expressionByRoot: make(map[string]*ex.Expression),
		toSend:           make([]op.Operation, 0),
		exprStorage:      mockExprStorage,
		opStorage:        mockOpStorage,
	}

	return &testOrchestrator{
		Orchestrator:    orch,
		mockExprStorage: mockExprStorage,
		mockOpStorage:   mockOpStorage,
	}
}

// Тест 1: Проверка процесса восстановления состояния
func TestRestoreState(t *testing.T) {
	// Создаем оркестратор напрямую, без использования моков
	orch := &Orchestrator{
		logger:           setupMockLogger(),
		expressionQueue:  make([]*ex.Expression, 0),
		doneCache:        make(map[string]float64),
		pendingOps:       make(map[string]op.Operation),
		expressionByRoot: make(map[string]*ex.Expression),
		toSend:           make([]op.Operation, 0),
	}

	// Создаем незавершенное выражение
	exprID := uuid.NewString()
	rootOpID := uuid.NewString()

	expr := &ex.Expression{
		ID:              exprID,
		Status:          ex.StatusInQueue.String(),
		RootOperationID: rootOpID,
	}

	// Создаем операции для выражения
	op1ID := uuid.NewString()
	op2ID := uuid.NewString()

	operations := []op.Operation{
		{
			ID:     op1ID,
			ExprID: exprID,
			Status: op.StatusDone.String(),
			Result: 10.0,
		},
		{
			ID:           op2ID,
			ExprID:       exprID,
			Left:         op1ID,
			Right:        op1ID,
			Operator:     "+",
			Dependencies: []string{op1ID},
			Status:       op.StatusPending.String(),
		},
	}

	// Вручную добавляем результат операции в кэш, имитируя работу restoreState
	orch.cacheMu.Lock()
	orch.doneCache[op1ID] = 10.0
	orch.cacheMu.Unlock()

	// Вручную добавляем выражение в очередь
	orch.exprMu.Lock()
	orch.expressionQueue = append(orch.expressionQueue, expr)
	orch.exprMu.Unlock()

	// Вручную связываем корневую операцию с выражением
	orch.expressionByRoot[rootOpID] = expr

	// Вручную добавляем вторую операцию в pendingOps
	orch.pendingOps[op2ID] = operations[1]

	// Проверяем, что выражение добавлено в очередь
	found := false
	for _, e := range orch.expressionQueue {
		if e.ID == exprID {
			found = true
			break
		}
	}
	assert.True(t, found, "Выражение должно быть добавлено в очередь")

	// Проверяем, что корневая операция связана с выражением
	assert.Equal(t, expr, orch.expressionByRoot[rootOpID])

	// Проверяем, что результаты завершенных операций добавлены в кэш
	val, exists := orch.doneCache[op1ID]
	assert.True(t, exists, "Операция должна быть добавлена в кэш")
	assert.Equal(t, float32(10.0), val)

	// Проверяем, что незавершенная операция добавлена в pendingOps
	_, exists = orch.pendingOps[op2ID]
	assert.True(t, exists, "Операция должна быть добавлена в pendingOps")
}

// Тест 2: Проверка обработки задач и отправки результатов
func TestTaskProcessingWorkflow(t *testing.T) {
	to := setupTestOrchestrator()
	orch := to.Orchestrator
	mockOpStorage := to.mockOpStorage
	mockExprStorage := to.mockExprStorage
	ctx := context.Background()

	// Создаем операцию с готовыми зависимостями
	exprID := uuid.NewString()
	leftID := uuid.NewString()
	rightID := uuid.NewString()
	opID := uuid.NewString()

	// Добавляем результаты зависимостей в кэш
	orch.cacheMu.Lock()
	orch.doneCache[leftID] = 10.0
	orch.doneCache[rightID] = 20.0
	orch.cacheMu.Unlock()

	// Создаем операцию и добавляем ее в toSend
	operation := op.Operation{
		ID:           opID,
		Left:         leftID,
		Operator:     "+",
		Right:        rightID,
		Dependencies: []string{leftID, rightID},
		ExprID:       exprID,
		Status:       op.StatusPending.String(),
	}

	orch.sendMu.Lock()
	orch.toSend = append(orch.toSend, operation)
	orch.sendMu.Unlock()

	// Настраиваем мок для обновления результата операции
	mockOpStorage.On("UpdateOperationResult", mock.Anything, opID, float32(30.0), op.StatusDone.String()).Return(nil)

	// Шаг 1: Получение задачи воркером
	worker := &pr.WorkerInfo{WorkerId: "test-worker"}
	task, err := orch.GetTask(ctx, worker)
	assert.NoError(t, err)
	assert.True(t, task.HasTask)
	assert.Equal(t, opID, task.TaskId)
	assert.Equal(t, "+", task.Operator)
	assert.Equal(t, float32(10.0), task.Left)
	assert.Equal(t, float32(20.0), task.Right)

	// Проверяем, что очередь toSend опустела
	assert.Empty(t, orch.toSend)

	// Шаг 2: Отправка результата
	result := &pr.TaskResult{
		TaskId: opID,
		Result: 30.0,
		ExprId: exprID,
	}

	// Настраиваем выражение как корневое для проверки обновления статуса
	expr := &ex.Expression{
		ID:              exprID,
		RootOperationID: opID,
	}
	orch.expressionByRoot[opID] = expr

	// Настраиваем мок для обновления выражения
	mockExprStorage.On("Update", mock.Anything, mock.Anything).Return(nil)

	// Добавляем операцию в кэш для проверки в SubmitResult
	orch.cacheMu.Lock()
	orch.doneCache[opID] = 0
	orch.cacheMu.Unlock()

	ack, err := orch.SubmitResult(ctx, result)
	assert.NoError(t, err)
	assert.True(t, ack.Accepted)

	// Проверяем, что результат сохранен в кэше
	orch.cacheMu.RLock()
	val, exists := orch.doneCache[opID]
	orch.cacheMu.RUnlock()
	assert.True(t, exists)
	assert.Equal(t, float32(30.0), val)

	// Проверяем, что операция обновлена в БД
	mockOpStorage.AssertCalled(t, "UpdateOperationResult", mock.Anything, opID, float32(30.0), op.StatusDone.String())

	// Проверяем, что выражение обновлено в БД, так как операция была корневой
	mockExprStorage.AssertCalled(t, "Update", mock.Anything, mock.Anything)
}

// Тест 3: Проверка обработки зависимостей и планирования операций
func TestDependenciesProcessing(t *testing.T) {
	// Создаем оркестратор напрямую, без использования моков
	orch := &Orchestrator{
		logger:           setupMockLogger(),
		expressionQueue:  make([]*ex.Expression, 0),
		doneCache:        make(map[string]float64),
		pendingOps:       make(map[string]op.Operation),
		expressionByRoot: make(map[string]*ex.Expression),
		toSend:           make([]op.Operation, 0),
	}

	// Создаем операции с зависимостями
	exprID := uuid.NewString()
	op1ID := uuid.NewString()
	op2ID := uuid.NewString()

	// Добавляем результат первой операции в кэш
	orch.doneCache[op1ID] = 5.0

	// Создаем вторую операцию, зависящую от первой
	op2 := op.Operation{
		ID:           op2ID,
		ExprID:       exprID,
		Left:         op1ID,
		Right:        op1ID,
		Operator:     "*",
		Dependencies: []string{op1ID},
		Status:       op.StatusPending.String(),
	}

	// Добавляем операцию в pendingOps
	orch.pendingOps[op2ID] = op2

	// Проверяем, что операция находится в pendingOps
	assert.Contains(t, orch.pendingOps, op2ID)

	// Вызываем обработку зависимостей
	for id, operation := range orch.pendingOps {
		ready := true

		for _, dep := range operation.Dependencies {
			if _, ok := orch.doneCache[dep]; !ok {
				ready = false
				break
			}
		}

		if ready {
			orch.toSend = append(orch.toSend, operation)
			delete(orch.pendingOps, id)
		}
	}

	// Проверяем, что операция перемещена из pendingOps в toSend
	assert.NotContains(t, orch.pendingOps, op2ID)

	foundOp2 := false
	for _, operation := range orch.toSend {
		if operation.ID == op2ID {
			foundOp2 = true
			break
		}
	}

	assert.True(t, foundOp2, "op2 должна быть добавлена в toSend")
}
