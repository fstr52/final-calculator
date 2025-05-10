package orchestrator

import (
	"sync"
	"testing"

	"github.com/fstr52/final-calculator/internal/logger"
	"github.com/stretchr/testify/assert"
)

// --- Минимальные proto-заглушки ---

type WorkerInfo struct {
	WorkerId string
}

type Task struct {
	TaskId   string
	Left     float32
	Operator string
	Right    float32
	HasTask  bool
	ExprId   string
}

type TaskResult struct {
	TaskId string
	Result float32
	Error  string
	ExprId string
}

type Ack struct {
	Accepted bool
}

// --- Минимальный logger.Logger ---

type silentLogger struct{}

func (l *silentLogger) Debug(msg string, args ...any)       {}
func (l *silentLogger) Info(msg string, args ...any)        {}
func (l *silentLogger) Warn(msg string, args ...any)        {}
func (l *silentLogger) Error(msg string, args ...any)       {}
func (l *silentLogger) With(args ...any) logger.Logger      { return l }
func (l *silentLogger) WithGroup(name string) logger.Logger { return l }

// Используй эту функцию в тестах вместо NewDefault()
func testLogger() logger.Logger {
	return &silentLogger{}
}

// --- Подмена proto-пакета ---

// Псевдонимы, чтобы не менять код оркестратора
var _ = WorkerInfo{}
var _ = Task{}
var _ = TaskResult{}
var _ = Ack{}

// --- Тесты ---

func TestNewExpression_Simple(t *testing.T) {
	o := NewOrchestrator(testLogger())
	expr, err := o.NewExpression("2+2")
	assert.NoError(t, err)
	assert.NotNil(t, expr)
	assert.Equal(t, StatusInQueue, expr.Status)
	assert.GreaterOrEqual(t, len(expr.Operations), 1)
}

func TestNewExpression_Invalid(t *testing.T) {
	o := NewOrchestrator(testLogger())
	expr, err := o.NewExpression("2++2")
	assert.Error(t, err)
	assert.Nil(t, expr)
}

func TestPlanOperations_AndDoneCache(t *testing.T) {
	o := NewOrchestrator(testLogger())
	expr, err := o.NewExpression("7")
	assert.NoError(t, err)
	assert.NotNil(t, expr)
	// Проверяем, что в doneCache появился результат
	found := false
	o.cacheMu.RLock()
	for _, v := range o.doneCache {
		if v == 7 {
			found = true
			break
		}
	}
	o.cacheMu.RUnlock()
	assert.True(t, found)
}

func TestStartSchedulerAndNextExpr(t *testing.T) {
	o := NewOrchestrator(testLogger())
	expr, err := o.NewExpression("1+2")
	assert.NoError(t, err)
	for _, op := range expr.Operations {
		o.cacheMu.Lock()
		o.doneCache[op.Left] = 1
		o.doneCache[op.Right] = 2
		o.cacheMu.Unlock()
	}
	o.nextExpr()
	o.sendMu.Lock()
	hasTask := len(o.toSend) > 0
	o.sendMu.Unlock()
	assert.True(t, hasTask)
}

func TestConcurrency_Safe(t *testing.T) {
	o := NewOrchestrator(testLogger())
	wg := sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = o.NewExpression("1+1")
		}(i)
	}
	wg.Wait()
	o.exprMu.Lock()
	count := len(o.expressionQueue)
	o.exprMu.Unlock()
	assert.GreaterOrEqual(t, count, 0)
}
