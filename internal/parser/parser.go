package parser

import (
	"fmt"
	"strings"

	"github.com/name-cho/uniland-go/internal/ast"
	"github.com/name-cho/uniland-go/internal/interp/val"
	"github.com/name-cho/uniland-go/internal/lexer"
	"github.com/name-cho/uniland-go/internal/unierr"
)

var assignOps = map[string]bool{"=": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true}

var precedence = map[string]int{
	"||": -1, "&&": -1,
	"==": 0, "!=": 0,
	"<": 1, "<=": 1, ">": 1, ">=": 1,
	"+": 2, "-": 2,
	"*": 3, "/": 3, "%": 3,
	"^": 4,
}

type Parser struct {
	tokens []lexer.Token
	pos    int
}

func Parse(tokens []lexer.Token) (*ast.Program, error) {
	p := &Parser{tokens: tokens}
	stmts, err := p.program()
	if err != nil {
		return nil, err
	}
	return &ast.Program{Statements: stmts}, nil
}

func eofToken() lexer.Token { return lexer.Token{Type: lexer.TokenEOF} }

func (p *Parser) peek(k int) lexer.Token {
	idx := p.pos + k
	if idx < len(p.tokens) {
		return p.tokens[idx]
	}
	return eofToken()
}

func (p *Parser) cur() lexer.Token { return p.peek(0) }

func (p *Parser) nextToken() lexer.Token {
	t := p.cur()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return t
}

func (p *Parser) expect(typ, value string) (lexer.Token, error) {
	t := p.cur()
	if t.Type != typ || (value != "" && t.Value != value) {
		want := value
		if want == "" {
			want = typ
		}
		return t, unierr.AtLine(fmt.Sprintf("Expected %s, got %s('%s')", want, t.Type, t.Value), t.Line)
	}
	p.nextToken()
	return t, nil
}

func (p *Parser) skipNewlines() {
	for p.cur().Type == lexer.TokenNewline {
		p.nextToken()
	}
}

func (p *Parser) isPunct(v string) bool {
	t := p.cur()
	return t.Type == lexer.TokenPunct && t.Value == v
}

func (p *Parser) isKw(v string) bool {
	t := p.cur()
	return t.Type == lexer.TokenKeyword && t.Value == v
}

func (p *Parser) endStatement() error {
	t := p.cur()
	if t.Type == lexer.TokenPunct && t.Value == ";" {
		p.nextToken()
		p.skipNewlines()
		return nil
	}
	if t.Type == lexer.TokenNewline {
		p.skipNewlines()
		return nil
	}
	if t.Type == lexer.TokenEOF || (t.Type == lexer.TokenPunct && t.Value == "}") {
		return nil
	}
	return unierr.AtLine(fmt.Sprintf("Expected end of statement, got %s('%s')", t.Type, t.Value), t.Line)
}

func (p *Parser) program() ([]ast.Node, error) {
	var stmts []ast.Node
	p.skipNewlines()
	for p.cur().Type != lexer.TokenEOF {
		stmt, err := p.statement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		p.skipNewlines()
	}
	return stmts, nil
}

func (p *Parser) ParseExpressionOnly() (ast.Node, error) {
	p.skipNewlines()
	return p.expression(-2)
}

func (p *Parser) statement() (ast.Node, error) {
	p.skipNewlines()
	t := p.cur()

	if t.Type == lexer.TokenKeyword {
		switch t.Value {
		case "func", "fn":
			return p.functionDeclaration()
		case "if":
			return p.ifStatement()
		case "for":
			return p.forStatement()
		case "while":
			return p.whileStatement()
		case "try":
			return p.tryStatement()
		}
	}
	if t.Type == lexer.TokenPunct && t.Value == "{" {
		return p.block()
	}
	if t.Type == lexer.TokenPyBlock {
		p.nextToken()
		node := &ast.PythonBlock{Code: t.Value, Line: t.Line}
		if err := p.endStatement(); err != nil {
			return nil, err
		}
		return node, nil
	}
	if t.Type == lexer.TokenGoBlock {
		p.nextToken()
		node := &ast.GoBlock{Code: t.Value, Line: t.Line}
		if err := p.endStatement(); err != nil {
			return nil, err
		}
		return node, nil
	}

	node, err := p.small()
	if err != nil {
		return nil, err
	}
	if err := p.endStatement(); err != nil {
		return nil, err
	}
	return node, nil
}

func (p *Parser) small() (ast.Node, error) {
	t := p.cur()
	if t.Type == lexer.TokenKeyword {
		switch t.Value {
		case "let", "const":
			return p.variableDeclaration()
		case "return":
			p.nextToken()
			nt := p.cur()
			if nt.Type == lexer.TokenNewline || nt.Type == lexer.TokenEOF ||
				(nt.Type == lexer.TokenPunct && (nt.Value == ";" || nt.Value == "}")) {
				return &ast.ReturnStatement{Line: t.Line}, nil
			}
			v, err := p.expression(-2)
			if err != nil {
				return nil, err
			}
			return &ast.ReturnStatement{Value: v, Line: t.Line}, nil
		case "break":
			p.nextToken()
			return &ast.BreakStatement{Line: t.Line}, nil
		case "continue":
			p.nextToken()
			return &ast.ContinueStatement{Line: t.Line}, nil
		case "throw":
			p.nextToken()
			v, err := p.expression(-2)
			if err != nil {
				return nil, err
			}
			return &ast.ThrowStatement{Value: v, Line: t.Line}, nil
		}
	}

	expr, err := p.expression(-2)
	if err != nil {
		return nil, err
	}
	op := p.cur()
	if op.Type == lexer.TokenOperator && assignOps[op.Value] {
		p.nextToken()
		v, err := p.expression(-2)
		if err != nil {
			return nil, err
		}
		return &ast.Assignment{Target: expr, Value: v, Op: op.Value, Line: op.Line}, nil
	}
	if op.Type == lexer.TokenOperator && (op.Value == "++" || op.Value == "--") {
		p.nextToken()
		return &ast.Assignment{Target: expr, Value: &ast.Literal{Value: val.Int64(1), Line: op.Line}, Op: op.Value, Line: op.Line}, nil
	}
	return &ast.ExpressionStatement{Expr: expr, Line: t.Line}, nil
}

func (p *Parser) block() (*ast.Block, error) {
	if _, err := p.expect(lexer.TokenPunct, "{"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	var stmts []ast.Node
	for !p.isPunct("}") && p.cur().Type != lexer.TokenEOF {
		stmt, err := p.statement()
		if err != nil {
			return nil, err
		}
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		p.skipNewlines()
	}
	if _, err := p.expect(lexer.TokenPunct, "}"); err != nil {
		return nil, err
	}
	return &ast.Block{Statements: stmts}, nil
}

func (p *Parser) variableDeclaration() (ast.Node, error) {
	isConst := p.cur().Value == "const"
	p.nextToken()
	ident, err := p.expect(lexer.TokenIdentifier, "")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenOperator, "="); err != nil {
		return nil, err
	}
	v, err := p.expression(-2)
	if err != nil {
		return nil, err
	}
	return &ast.VariableDeclaration{Name: ident.Value, Value: v, IsConst: isConst, Line: ident.Line}, nil
}

func (p *Parser) functionDeclaration() (ast.Node, error) {
	t := p.cur()
	p.nextToken()
	name, err := p.expect(lexer.TokenIdentifier, "")
	if err != nil {
		return nil, err
	}
	params, err := p.paramList()
	if err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	return &ast.FunctionDeclaration{Name: name.Value, Params: params, Body: body, Line: t.Line}, nil
}

func (p *Parser) paramList() ([]string, error) {
	if _, err := p.expect(lexer.TokenPunct, "("); err != nil {
		return nil, err
	}
	p.skipNewlines()
	var params []string
	if !p.isPunct(")") {
		for {
			ident, err := p.expect(lexer.TokenIdentifier, "")
			if err != nil {
				return nil, err
			}
			params = append(params, ident.Value)
			p.skipNewlines()
			if p.isPunct(",") {
				p.nextToken()
				p.skipNewlines()
				continue
			}
			break
		}
	}
	if _, err := p.expect(lexer.TokenPunct, ")"); err != nil {
		return nil, err
	}
	return params, nil
}

func (p *Parser) ifStatement() (ast.Node, error) {
	line := p.cur().Line
	p.nextToken()
	if _, err := p.expect(lexer.TokenPunct, "("); err != nil {
		return nil, err
	}
	cond, err := p.expression(-2)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenPunct, ")"); err != nil {
		return nil, err
	}
	thenB, err := p.block()
	if err != nil {
		return nil, err
	}
	var elseB ast.Node
	p.skipNewlines()
	if p.isKw("else") {
		p.nextToken()
		p.skipNewlines()
		if p.isKw("if") {
			elseB, err = p.ifStatement()
		} else {
			elseB, err = p.block()
		}
		if err != nil {
			return nil, err
		}
	}
	return &ast.IfStatement{Condition: cond, Then: thenB, Else: elseB, Line: line}, nil
}

func (p *Parser) forStatement() (ast.Node, error) {
	line := p.cur().Line
	p.nextToken()
	if _, err := p.expect(lexer.TokenPunct, "("); err != nil {
		return nil, err
	}

	if p.isForIn() {
		if p.isKw("let") || p.isKw("const") {
			p.nextToken()
		}
		varTok, err := p.expect(lexer.TokenIdentifier, "")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenKeyword, "in"); err != nil {
			return nil, err
		}
		iterable, err := p.expression(-2)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(lexer.TokenPunct, ")"); err != nil {
			return nil, err
		}
		body, err := p.block()
		if err != nil {
			return nil, err
		}
		return &ast.ForInLoop{Var: varTok.Value, Iterable: iterable, Body: body, Line: line}, nil
	}

	var init ast.Node
	if !p.isPunct(";") {
		n, err := p.small()
		if err != nil {
			return nil, err
		}
		init = n
	}
	if _, err := p.expect(lexer.TokenPunct, ";"); err != nil {
		return nil, err
	}
	var cond ast.Node = &ast.Literal{Value: val.Bool(true), Line: line}
	if !p.isPunct(";") {
		c, err := p.expression(-2)
		if err != nil {
			return nil, err
		}
		cond = c
	}
	if _, err := p.expect(lexer.TokenPunct, ";"); err != nil {
		return nil, err
	}
	var update ast.Node
	if !p.isPunct(")") {
		u, err := p.small()
		if err != nil {
			return nil, err
		}
		update = u
	}
	if _, err := p.expect(lexer.TokenPunct, ")"); err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	return &ast.ForLoop{Init: init, Condition: cond, Update: update, Body: body, Line: line}, nil
}

func (p *Parser) isForIn() bool {
	if p.isKw("let") || p.isKw("const") {
		return p.peek(1).Type == lexer.TokenIdentifier &&
			p.peek(2).Type == lexer.TokenKeyword && p.peek(2).Value == "in"
	}
	if p.cur().Type == lexer.TokenIdentifier {
		return p.peek(1).Type == lexer.TokenKeyword && p.peek(1).Value == "in"
	}
	return false
}

func (p *Parser) whileStatement() (ast.Node, error) {
	line := p.cur().Line
	p.nextToken()
	if _, err := p.expect(lexer.TokenPunct, "("); err != nil {
		return nil, err
	}
	cond, err := p.expression(-2)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(lexer.TokenPunct, ")"); err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	return &ast.WhileLoop{Condition: cond, Body: body, Line: line}, nil
}

func (p *Parser) tryStatement() (ast.Node, error) {
	line := p.cur().Line
	p.nextToken()
	tryB, err := p.block()
	if err != nil {
		return nil, err
	}
	var catchVar *string
	var catchB *ast.Block
	p.skipNewlines()
	if p.isKw("catch") {
		p.nextToken()
		if p.isPunct("(") {
			p.nextToken()
			ident, err := p.expect(lexer.TokenIdentifier, "")
			if err != nil {
				return nil, err
			}
			v := ident.Value
			catchVar = &v
			if _, err := p.expect(lexer.TokenPunct, ")"); err != nil {
				return nil, err
			}
		}
		cb, err := p.block()
		if err != nil {
			return nil, err
		}
		catchB = cb
	}
	p.skipNewlines()
	var finallyB *ast.Block
	if p.isKw("finally") {
		p.nextToken()
		fb, err := p.block()
		if err != nil {
			return nil, err
		}
		finallyB = fb
	}
	return &ast.TryCatch{Try: tryB, CatchVar: catchVar, Catch: catchB, Finally: finallyB, Line: line}, nil
}

func (p *Parser) expression(minPrec int) (ast.Node, error) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.cur()
		if t.Type != lexer.TokenOperator {
			break
		}
		prec, ok := precedence[t.Value]
		if !ok || prec < minPrec {
			break
		}
		p.nextToken()
		p.skipNewlines()
		right, err := p.expression(prec + 1)
		if err != nil {
			return nil, err
		}
		left = &ast.BinaryOp{Left: left, Right: right, Op: t.Value, Line: t.Line}
	}
	return left, nil
}

func (p *Parser) unary() (ast.Node, error) {
	t := p.cur()
	if t.Type == lexer.TokenOperator && (t.Value == "-" || t.Value == "!") {
		p.nextToken()
		e, err := p.unary()
		if err != nil {
			return nil, err
		}
		return &ast.UnaryOp{Op: t.Value, Expr: e, Line: t.Line}, nil
	}
	return p.postfix()
}

func (p *Parser) postfix() (ast.Node, error) {
	node, err := p.primary()
	if err != nil {
		return nil, err
	}
	for {
		t := p.cur()
		if p.isPunct("(") {
			p.nextToken()
			p.skipNewlines()
			var args []ast.Node
			if !p.isPunct(")") {
				for {
					a, err := p.expression(-2)
					if err != nil {
						return nil, err
					}
					args = append(args, a)
					p.skipNewlines()
					if p.isPunct(",") {
						p.nextToken()
						p.skipNewlines()
						continue
					}
					break
				}
			}
			if _, err := p.expect(lexer.TokenPunct, ")"); err != nil {
				return nil, err
			}
			node = &ast.CallExpression{Callee: node, Args: args, Line: t.Line}
		} else if p.isPunct("[") {
			p.nextToken()
			idx, err := p.expression(-2)
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(lexer.TokenPunct, "]"); err != nil {
				return nil, err
			}
			node = &ast.IndexExpression{Obj: node, Index: idx, Line: t.Line}
		} else if p.isPunct(".") {
			p.nextToken()
			name, err := p.expect(lexer.TokenIdentifier, "")
			if err != nil {
				return nil, err
			}
			node = &ast.MemberExpression{Obj: node, Name: name.Value, Line: t.Line}
		} else {
			break
		}
	}
	return node, nil
}

func (p *Parser) primary() (ast.Node, error) {
	t := p.cur()

	if t.Type == lexer.TokenIdentifier && p.peek(1).Type == lexer.TokenOperator && p.peek(1).Value == "=>" {
		p.nextToken()
		p.nextToken()
		body, err := p.arrowBody()
		if err != nil {
			return nil, err
		}
		return &ast.FunctionExpression{Params: []string{t.Value}, Body: body, Line: t.Line}, nil
	}

	switch t.Type {
	case lexer.TokenNumber:
		p.nextToken()
		return &ast.Literal{Value: val.IntOrFloat(t.Value), Line: t.Line}, nil

	case lexer.TokenString:
		p.nextToken()
		return &ast.Literal{Value: val.Str(t.Value), Line: t.Line}, nil

	case lexer.TokenTemplate:
		p.nextToken()
		return p.buildTemplate([]rune(t.Value), t.Line)

	case lexer.TokenIdentifier:
		p.nextToken()
		return &ast.Identifier{Name: t.Value, Line: t.Line}, nil

	case lexer.TokenPyBlock:
		return nil, unierr.AtLine("python {} block cannot be used as an expression", t.Line)

	case lexer.TokenKeyword:
		switch t.Value {
		case "true":
			p.nextToken()
			return &ast.Literal{Value: val.Bool(true), Line: t.Line}, nil
		case "false":
			p.nextToken()
			return &ast.Literal{Value: val.Bool(false), Line: t.Line}, nil
		case "null":
			p.nextToken()
			return &ast.Literal{Value: val.Nul, Line: t.Line}, nil
		case "func", "fn":
			p.nextToken()
			params, err := p.paramList()
			if err != nil {
				return nil, err
			}
			body, err := p.block()
			if err != nil {
				return nil, err
			}
			return &ast.FunctionExpression{Params: params, Body: body, Line: t.Line}, nil
		default:

			p.nextToken()
			return &ast.Identifier{Name: t.Value, Line: t.Line}, nil
		}

	case lexer.TokenPunct:
		switch t.Value {
		case "(":
			params, ok := p.tryArrowParams()
			if ok {
				body, err := p.arrowBody()
				if err != nil {
					return nil, err
				}
				return &ast.FunctionExpression{Params: params, Body: body, Line: t.Line}, nil
			}
			p.nextToken()
			p.skipNewlines()
			node, err := p.expression(-2)
			if err != nil {
				return nil, err
			}
			p.skipNewlines()
			if _, err := p.expect(lexer.TokenPunct, ")"); err != nil {
				return nil, err
			}
			return node, nil
		case "[":
			return p.arrayLiteral()
		case "{":
			return p.objectLiteral()
		}
	}

	return nil, unierr.AtLine(fmt.Sprintf("Unexpected token %s('%s')", t.Type, t.Value), t.Line)
}

func (p *Parser) arrayLiteral() (ast.Node, error) {
	line := p.cur().Line
	if _, err := p.expect(lexer.TokenPunct, "["); err != nil {
		return nil, err
	}
	p.skipNewlines()
	var elems []ast.Node
	if !p.isPunct("]") {
		for {
			e, err := p.expression(-2)
			if err != nil {
				return nil, err
			}
			elems = append(elems, e)
			p.skipNewlines()
			if p.isPunct(",") {
				p.nextToken()
				p.skipNewlines()
				continue
			}
			break
		}
	}
	if _, err := p.expect(lexer.TokenPunct, "]"); err != nil {
		return nil, err
	}
	return &ast.ArrayLiteral{Elements: elems, Line: line}, nil
}

func (p *Parser) objectLiteral() (ast.Node, error) {
	line := p.cur().Line
	if _, err := p.expect(lexer.TokenPunct, "{"); err != nil {
		return nil, err
	}
	p.skipNewlines()
	var pairs []ast.KeyValue
	if !p.isPunct("}") {
		for {
			kt := p.cur()
			var key string
			if kt.Type == lexer.TokenIdentifier || kt.Type == lexer.TokenString {
				key = kt.Value
				p.nextToken()
			} else {
				return nil, unierr.AtLine("Expected identifier or string as object key", kt.Line)
			}
			if _, err := p.expect(lexer.TokenPunct, ":"); err != nil {
				return nil, err
			}
			v, err := p.expression(-2)
			if err != nil {
				return nil, err
			}
			pairs = append(pairs, ast.KeyValue{Key: key, Value: v})
			p.skipNewlines()
			if p.isPunct(",") {
				p.nextToken()
				p.skipNewlines()
				continue
			}
			break
		}
	}
	p.skipNewlines()
	if _, err := p.expect(lexer.TokenPunct, "}"); err != nil {
		return nil, err
	}
	return &ast.ObjectLiteral{Pairs: pairs, Line: line}, nil
}

func (p *Parser) arrowBody() (*ast.Block, error) {
	p.skipNewlines()
	if p.isPunct("{") {
		return p.block()
	}
	expr, err := p.expression(-2)
	if err != nil {
		return nil, err
	}
	return &ast.Block{Statements: []ast.Node{&ast.ReturnStatement{Value: expr}}}, nil
}

func (p *Parser) tryArrowParams() ([]string, bool) {
	save := p.pos
	if !p.isPunct("(") {
		return nil, false
	}
	p.nextToken()
	out := []string{}
	if !p.isPunct(")") {
		for {
			if p.cur().Type != lexer.TokenIdentifier {
				p.restore(save)
				return nil, false
			}
			out = append(out, p.cur().Value)
			p.nextToken()
			if p.isPunct(",") {
				p.nextToken()
				continue
			}
			break
		}
	}
	if !p.isPunct(")") {
		p.restore(save)
		return nil, false
	}
	p.nextToken()
	if !(p.cur().Type == lexer.TokenOperator && p.cur().Value == "=>") {
		p.restore(save)
		return nil, false
	}
	p.nextToken()
	return out, true
}

func (p *Parser) restore(pos int) { p.pos = pos }

func (p *Parser) buildTemplate(raw []rune, line int) (ast.Node, error) {
	var parts []ast.TPLPart
	var buf []rune
	i, n := 0, len(raw)
	for i < n {
		ch := raw[i]
		if ch == '{' && i+1 < n && raw[i+1] == '{' {
			buf = append(buf, '{')
			i += 2
			continue
		}
		if ch == '}' && i+1 < n && raw[i+1] == '}' {
			buf = append(buf, '}')
			i += 2
			continue
		}
		if ch == '{' {
			if len(buf) > 0 {
				parts = append(parts, ast.StrPart(string(buf)))
				buf = nil
			}
			depth := 1
			j := i + 1
			var exprChars []rune
			for j < n && depth > 0 {
				cj := raw[j]
				if cj == '{' {
					depth++
				} else if cj == '}' {
					depth--
					if depth == 0 {
						break
					}
				}
				exprChars = append(exprChars, cj)
				j++
			}
			exprSrc := strings.TrimSpace(string(exprChars))
			subTokens, err := lexer.Tokenize(exprSrc)
			if err != nil {
				return nil, err
			}
			sub, err := (&Parser{tokens: subTokens}).ParseExpressionOnly()
			if err != nil {
				return nil, err
			}
			parts = append(parts, ast.ExprPart(sub))
			i = j + 1
		} else {
			buf = append(buf, ch)
			i++
		}
	}
	if len(buf) > 0 {
		parts = append(parts, ast.StrPart(string(buf)))
	}
	return &ast.TemplateLiteral{Parts: parts, Line: line}, nil
}
