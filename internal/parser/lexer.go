package parser

import (
	"fmt"
	"unicode"

	"github.com/fstr52/final-calculator/internal/logger"
)

type TokenType string

const (
	TokenEOF    TokenType = "EOF"
	TokenNum    TokenType = "NUM"
	TokenPlus   TokenType = "+"
	TokenMinus  TokenType = "-"
	TokenStar   TokenType = "*"
	TokenSlash  TokenType = "/"
	TokenLParen TokenType = "("
	TokenRParen TokenType = ")"
	TokenEmpty  TokenType = ""
)

type Token struct {
	Type  TokenType
	Value string
}

type Lexer struct {
	logger logger.Logger
	input  []rune
	pos    int
}

func NewLexer(input string, logger logger.Logger) *Lexer {
	return &Lexer{
		logger: logger,
		input:  []rune(input),
	}
}

func (l *Lexer) Next() (Token, error) {
	l.logger.Debug("Started l.Next")

	for l.pos < len(l.input) && unicode.IsSpace(l.input[l.pos]) {
		l.pos++
	}

	if l.pos >= len(l.input) {
		return Token{Type: TokenEOF}, nil
	}

	sym := l.input[l.pos]

	switch sym {
	case '+':
		l.pos++
		return Token{Type: TokenPlus, Value: "+"}, nil
	case '-':
		l.pos++
		return Token{Type: TokenMinus, Value: "-"}, nil
	case '*':
		l.pos++
		return Token{Type: TokenStar, Value: "*"}, nil
	case '/':
		l.pos++
		return Token{Type: TokenSlash, Value: "/"}, nil
	case '(':
		l.pos++
		return Token{Type: TokenLParen, Value: "("}, nil
	case ')':
		l.pos++
		return Token{Type: TokenRParen, Value: ")"}, nil
	}

	if unicode.IsDigit(sym) {
		start := l.pos
		hasDot := false
		for l.pos < len(l.input) && (unicode.IsDigit(l.input[l.pos]) || l.input[l.pos] == '.' || l.input[l.pos] == ',') {
			if l.input[l.pos] == '.' || l.input[l.pos] == ',' {
				if hasDot {
					l.logger.Debug("Too many dots")
					return Token{}, fmt.Errorf("too many dots")
				}

				l.input[l.pos] = '.'
			}
			l.pos++
		}
		return Token{Type: TokenNum, Value: string(l.input[start:l.pos])}, nil
	}

	l.logger.Debug("Unexpected symbol",
		"sym", sym)
	return Token{}, fmt.Errorf("unexpected symbol: %v", sym)
}
