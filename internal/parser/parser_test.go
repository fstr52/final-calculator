package parser

import (
	"testing"

	"github.com/fstr52/final-calculator/internal/logger"
	"github.com/stretchr/testify/assert"
)

func TestParser_ParseExpr(t *testing.T) {
	mockLogger := &MockLogger{}

	tests := []struct {
		name     string
		input    string
		expected float32
		wantErr  bool
	}{
		{
			name:     "Simple number",
			input:    "42",
			expected: 42.0,
			wantErr:  false,
		},
		{
			name:     "Addition",
			input:    "2 + 3",
			expected: 5.0,
			wantErr:  false,
		},
		{
			name:     "Subtraction",
			input:    "5 - 3",
			expected: 2.0,
			wantErr:  false,
		},
		{
			name:     "Multiplication",
			input:    "4 * 5",
			expected: 20.0,
			wantErr:  false,
		},
		{
			name:     "Division",
			input:    "20 / 4",
			expected: 5.0,
			wantErr:  false,
		},
		{
			name:     "Complex expression",
			input:    "2 + 3 * 4",
			expected: 14.0, // Проверка приоритета операций
			wantErr:  false,
		},
		{
			name:     "Complex expression with parentheses",
			input:    "(2 + 3) * 4",
			expected: 20.0,
			wantErr:  false,
		},
		{
			name:     "Unary minus",
			input:    "-5",
			expected: -5.0,
			wantErr:  false,
		},
		{
			name:     "Unary minus with expression",
			input:    "-5 + 3",
			expected: -2.0,
			wantErr:  false,
		},
		{
			name:     "Multiple operations",
			input:    "2 + 3 * 4 - 6 / 2",
			expected: 11.0, // 2 + 12 - 3 = 11
			wantErr:  false,
		},
		{
			name:     "Nested parentheses",
			input:    "((2 + 3) * (4 - 1)) / 3",
			expected: 5.0, // (5 * 3) / 3 = 5
			wantErr:  false,
		},
		{
			name:     "Decimal numbers",
			input:    "2.5 + 3.5",
			expected: 6.0,
			wantErr:  false,
		},
		{
			name:    "Invalid expression - missing operand",
			input:   "2 +",
			wantErr: true,
		},
		{
			name:    "Invalid expression - mismatched parentheses",
			input:   "(2 + 3",
			wantErr: true,
		},
		{
			name:    "Invalid expression - empty",
			input:   "",
			wantErr: true,
		},
		{
			name:    "Invalid expression - invalid character",
			input:   "2 @ 3",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser(tt.input, mockLogger)

			expr, err := p.ParseExpr()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, expr)

			result := expr.Eval()
			assert.InDelta(t, float64(tt.expected), float64(result), 0.0001)
		})
	}
}

func TestParser_OperatorPrecedence(t *testing.T) {
	mockLogger := &MockLogger{}

	tests := []struct {
		input    string
		expected float32
	}{
		{"1 + 2 * 3", 7},      // Умножение имеет больший приоритет
		{"(1 + 2) * 3", 9},    // Скобки изменяют приоритет
		{"1 + 2 - 3", 0},      // Сложение и вычитание имеют одинаковый приоритет
		{"6 / 2 * 3", 9},      // Деление и умножение имеют одинаковый приоритет
		{"6 / (2 * 3)", 1},    // Скобки изменяют приоритет
		{"1 + 2 + 3 * 4", 15}, // Смешанные операции
		{"3 * 4 + 2 * 5", 22}, // Умножение выполняется первым
		{"-3 * 4", -12},       // Унарный минус имеет высокий приоритет
		{"-(3 * 4)", -12},     // Унарный минус с скобками
		{"-(3 + 4) * 2", -14}, // Комбинация унарного минуса и скобок
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := NewParser(tt.input, mockLogger)
			expr, err := p.ParseExpr()

			assert.NoError(t, err)
			assert.NotNil(t, expr)

			result := expr.Eval()
			assert.InDelta(t, float64(tt.expected), float64(result), 0.0001)
		})
	}
}

type MockLogger struct{}

func (m *MockLogger) Debug(msg string, args ...any)       {}
func (m *MockLogger) Info(msg string, args ...any)        {}
func (m *MockLogger) Warn(msg string, args ...any)        {}
func (m *MockLogger) Error(msg string, args ...any)       {}
func (m *MockLogger) With(args ...any) logger.Logger      { return m }
func (m *MockLogger) WithGroup(name string) logger.Logger { return m }
