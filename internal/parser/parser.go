package parser

import (
	"fmt"
	"strconv"

	"github.com/fstr52/final-calculator/internal/logger"
)

type Expr interface {
	Eval() float32
}

type Number struct {
	Value float32
}

func (n *Number) Eval() float32 {
	return n.Value
}

type Binary struct {
	Left   Expr
	Symbol Token
	Right  Expr
}

func (b *Binary) Eval() float32 {
	switch b.Symbol.Type {
	case TokenPlus:
		return b.Left.Eval() + b.Right.Eval()
	case TokenMinus:
		return b.Left.Eval() - b.Right.Eval()
	case TokenStar:
		return b.Left.Eval() * b.Right.Eval()
	case TokenSlash:
		return b.Left.Eval() / b.Right.Eval()
	}

	return 0
}

type Parser struct {
	logger logger.Logger
	lexer  *Lexer
	curr   Token
}

func NewParser(input string, logger logger.Logger) *Parser {
	l := NewLexer(input, logger)
	return &Parser{
		logger: logger,
		lexer:  l,
		curr: Token{
			Type: TokenEmpty,
		},
	}
}

func (p *Parser) ParseExpr() (Expr, error) {
	p.logger.Debug("Started ParseExpr")

	if p.curr.Type == TokenEmpty {
		curr, err := p.lexer.Next()
		if err != nil {
			return nil, err
		}
		p.curr = curr
	}

	res, err := p.parseExpr(0)
	if err != nil {
		return nil, err
	}

	p.logger.Debug("ParseExpr finished")
	return res, nil
}

func (p *Parser) eat(t TokenType) error {
	p.logger.Debug("Started p.eat")
	if p.curr.Type == t {
		next, err := p.lexer.Next()
		if err != nil {

			return err
		}
		p.curr = next
		return nil
	} else {
		p.logger.Debug("Expected type != got",
			"expected", t,
			"got", p.curr.Type)
		return fmt.Errorf("expected %s, got %s", t, p.curr.Type)
	}
}

func (p *Parser) lbp(tok TokenType) int {
	switch tok {
	case TokenPlus, TokenMinus:
		return 10
	case TokenStar, TokenSlash:
		return 20
	default:
		return 0
	}
}

func (p *Parser) nud(tok Token) (Expr, error) {
	p.logger.Debug("Started p.nud",
		"tok", fmt.Sprintf("%+v", tok))
	switch tok.Type {
	case TokenNum:
		val, err := strconv.ParseFloat(tok.Value, 32)
		if err != nil {
			p.logger.Error("Failed to parseFloat",
				"val", val,
				"error", err)
			return nil, err
		}
		return &Number{Value: float32(val)}, nil
	case TokenLParen:
		expr, err := p.parseExpr(0)
		if err != nil {
			return nil, err
		}

		err = p.eat(TokenRParen)
		if err != nil {
			return nil, err
		}

		return expr, nil
	case TokenMinus:
		right, err := p.parseExpr(100)
		if err != nil {
			return nil, err
		}

		return &Binary{
			Left:   &Number{Value: 0},
			Symbol: tok,
			Right:  right,
		}, nil
	}

	p.logger.Debug("Unexpected token",
		"tok", fmt.Sprintf("%+v", tok))
	return nil, fmt.Errorf("unexpected token")
}

func (p *Parser) led(left Expr, tok Token) (Expr, error) {
	right, err := p.parseExpr(p.lbp(tok.Type))
	if err != nil {
		return nil, err
	}
	return &Binary{
		Left:   left,
		Symbol: tok,
		Right:  right,
	}, nil
}

func (p *Parser) parseExpr(rbp int) (Expr, error) {
	p.logger.Debug("Started p.parseExpr")
	var err error

	tok := p.curr
	p.curr, err = p.lexer.Next()
	if err != nil {
		return nil, err
	}
	left, err := p.nud(tok)
	if err != nil {
		return nil, err
	}

	for rbp < p.lbp(p.curr.Type) {
		tok := p.curr
		p.curr, err = p.lexer.Next()
		if err != nil {
			return nil, err
		}
		left, err = p.led(left, tok)
		if err != nil {
			return nil, err
		}
	}

	p.logger.Debug("parseExpr finished")
	return left, nil
}
