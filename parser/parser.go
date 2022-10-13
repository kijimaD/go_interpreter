// 構文解析器

package parser

import (
	"fmt"
	"monkey/ast"
	"monkey/lexer"
	"monkey/token"
	"strconv"
)

type Parser struct {
	l      *lexer.Lexer
	errors []string

	curToken  token.Token // 現在のトークン
	peekToken token.Token // 次のトークン

	// 構文解析関数がどちらの中置もしくは前置のマップにあるかをチェックする
	prefixParseFns map[token.TokenType]prefixParseFn
	infixParseFns  map[token.TokenType]infixParseFn
}

type (
	// どちらの関数もast.Expressionを返す。これが欲しいもの

	// 前置構文解析関数 ++1
	// 前置演算子には「左側」が存在しない
	prefixParseFn func() ast.Expression

	// 中置構文解析関数 n + 1
	// 引数は中置演算子の「左側」
	infixParseFn func(ast.Expression) ast.Expression
)

// iotaで割り当てられる整数の値は重要ではない。演算子の優先順位を表現するものとして重要。
const (
	_ int = iota
	LOWEST
	EQUALS      // ==
	LESSGREATER // > または <
	SUM         // +
	PRODUCT     // *
	PREFIX      // -X または !X
	CALL        // myFunction(X, Y), 関数呼び出しでは ( は中置演算子になる
)

// 優先順位テーブル。トークンタイプと優先順位を関連付ける
var precedences = map[token.TokenType]int{
	token.EQ:       EQUALS,
	token.NOT_EQ:   EQUALS,
	token.LT:       LESSGREATER,
	token.GT:       LESSGREATER,
	token.PLUS:     SUM,
	token.MINUS:    SUM,
	token.SLASH:    PRODUCT,
	token.ASTERISK: PRODUCT,
	token.LPAREN:   CALL,
}

// 字句解析器を受け取って初期化する
func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	// 前置演算子
	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier) // もしトークンタイプtoken.IDENTが出現したら、呼び出すべき構文解析関数はparseIdentifier
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)
	p.registerPrefix(token.TRUE, p.parseBoolean)
	p.registerPrefix(token.FALSE, p.parseBoolean)
	p.registerPrefix(token.LPAREN, p.parseGroupedExpression)
	p.registerPrefix(token.IF, p.parseIfExpression)
	p.registerPrefix(token.FUNCTION, p.parseFunctionLiteral)

	// 中置演算子
	p.infixParseFns = make(map[token.TokenType]infixParseFn)
	p.registerInfix(token.PLUS, p.parseInfixExpression)
	p.registerInfix(token.MINUS, p.parseInfixExpression)
	p.registerInfix(token.SLASH, p.parseInfixExpression)
	p.registerInfix(token.ASTERISK, p.parseInfixExpression)
	p.registerInfix(token.EQ, p.parseInfixExpression)
	p.registerInfix(token.NOT_EQ, p.parseInfixExpression)
	p.registerInfix(token.LT, p.parseInfixExpression)
	p.registerInfix(token.GT, p.parseInfixExpression)
	p.registerInfix(token.LPAREN, p.parseCallExpression)

	// 2つトークンを読み込む。curTokenとpeekTokenの両方がセットされる
	p.nextToken()
	p.nextToken()

	return p
}

// エラーのアクセサ
func (p *Parser) Errors() []string {
	return p.errors
}

// エラーを追加する
func (p *Parser) peekError(t token.TokenType) {
	msg := fmt.Sprintf("expected next token to be %s, got %s instead",
		t,
		p.peekToken.Type,
	)
	p.errors = append(p.errors, msg)
}

// 次のトークンに進む
func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

// パースを開始する。トークンを1つずつ辿る
func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	program.Statements = []ast.Statement{}

	for p.curToken.Type != token.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

// 文をパースする。トークンの型によって適用関数を変える
// Monkey言語では、文で構成されるのはこれだけ
func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case token.LET:
		return p.parseLetStatement()
	case token.RETURN:
		return p.parseReturnStatement()
	default:
		// 式文の構文解析を試みる
		return p.parseExpressionStatement()
	}
}

// let文をパースする
// IDENT -> ASSIGN -> SEMICOLON のトークン列を満たさない場合をアサーションexpectPeek()によって確認する
// 左から右に、次のトークンが期待通りであるか、そうでないかを判断しつつ、全てがぴったりはまったらASTノードを返す
func (p *Parser) parseLetStatement() *ast.LetStatement {
	// 現在見ているトークンに基づいてLetStatementノードを構築する
	// stmt => statement
	stmt := &ast.LetStatement{Token: p.curToken}

	// 後続するトークンにアサーションを設けつつトークンを進める
	if !p.expectPeek(token.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}

	if !p.expectPeek(token.ASSIGN) {
		return nil
	}

	// TODO: セミコロンに遭遇するまで式を読み飛ばしてしまっている
	for !p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// returnをパースする
func (p *Parser) parseReturnStatement() *ast.ReturnStatement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	p.nextToken()

	// TODO: セミコロンに遭遇するまで式を読み飛ばしてしまっている
	for !p.curTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// booleanをパースする
func (p *Parser) parseBoolean() ast.Expression {
	return &ast.Boolean{Token: p.curToken, Value: p.curTokenIs(token.TRUE)}
}

// 現在のトークンと引数の型を比較する
func (p *Parser) curTokenIs(t token.TokenType) bool {
	return p.curToken.Type == t
}

// 次のトークンと引数の型を比較する
func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

// peekTokenの型をチェックし、その型が正しい場合に限ってnextTokenを読んで、トークンを進める
func (p *Parser) expectPeek(t token.TokenType) bool {
	if p.peekTokenIs(t) {
		p.nextToken()
		return true
	} else {
		p.peekError(t)
		return false
	}
}

// 構文解析関数を登録する
func (p *Parser) registerPrefix(tokenType token.TokenType, fn prefixParseFn) {
	p.prefixParseFns[tokenType] = fn
}

// 構文解析関数を登録する
func (p *Parser) registerInfix(tokenType token.TokenType, fn infixParseFn) {
	p.infixParseFns[tokenType] = fn
}

// 式文を構文解析する
func (p *Parser) parseExpressionStatement() *ast.ExpressionStatement {
	defer untrace(trace("parseExpressionStatement"))
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	stmt.Expression = p.parseExpression(LOWEST)

	// セミコロンは省略可能。あとでREPLに入力しやすくなる
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// p.curToken.Typeの前置に関連付けられた構文解析関数があるかを確認し、存在していればその構文解析関数を呼び出し、その結果を返す
// やりたいこと: 高い優先順位を持つ演算子に関する式を、より低い優先順位を持つ演算子に関する式と比べて、より木の深いレベルに配置すること。引数precedenceはそのために使われる
// 1の優先度が高いとき
// / (1+2)+3
// /     ┃
// /   ┃━┃
// / ┃━┃ ┃
// / 1+2+3
//
// 3の優先度が高いとき
// / 1+(2+3)
// / ┃
// / ┃━┃
// / ┃ ┃━┃
// / 1+2+3
//
// / 前置演算子の場合。定義からPREFIXは高い優先順位を持つ。このため、parseExpression(PREFIX)は-1の中の1を構文解析しようとしてinfixParseFnに渡すことは決してない。どのinfixParseFnも1を左腕に取ることはなく、1は前置式の右腕として返される。
// / -1+2
// /     ┃
// /   ┃━┃
// / ┃━┃ ┃
// / - 1+2
func (p *Parser) parseExpression(precedence int) ast.Expression {
	defer untrace(trace("parseExpression"))
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

	// 優先順位の処理を行っている重要な部分
	// より低い優先順位のトークンに遭遇する間繰り返す
	// 優先順位が同じもしくは高いトークンに遭遇すると実行しない
	for !p.peekTokenIs(token.SEMICOLON) && precedence < p.peekPrecedence() {
		infix := p.infixParseFns[p.peekToken.Type]
		if infix == nil {
			return leftExp
		}

		p.nextToken()

		leftExp = infix(leftExp)
	}

	return leftExp
}

// 識別子パース。*ast.Identifierを返す
func (p *Parser) parseIdentifier() ast.Expression {
	// 現在のトークンをTokenフィールドに、トークンのリテラル値をValueフィールドに格納する
	// トークンは進めない
	return &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
}

// 整数パース
// p.curToken.Literalの文字列をint64に変換する
func (p *Parser) parseIntegerLiteral() ast.Expression {
	defer untrace(trace("parseIntegerLiteral"))
	lit := &ast.IntegerLiteral{Token: p.curToken}

	value, err := strconv.ParseInt(p.curToken.Literal, 0, 64)
	if err != nil {
		msg := fmt.Sprintf("could not parse %q as integer", p.curToken.Literal)
		p.errors = append(p.errors, msg)
		return nil
	}

	lit.Value = value

	return lit
}

// 前置式パース。ほかのパース関数と異なり、トークンが進むのに注意
func (p *Parser) parsePrefixExpression() ast.Expression {
	defer untrace(trace("parsePrefixExpression"))
	expression := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Literal,
	}

	// トークンを進める
	// parsePrefixExpressionが呼ばれるとき、p.curTokenはタイプtoken.BANGかタイプtoken.MINUSのいずれか。そうでなければこの関数が呼ばれることはないから。しかし、-5のような前置演算子式を正しく構文解析するには、複数のトークンが「消費」される必要がある。
	// そこで、*ast.PrefixExpressionノードを構築するためにp.curTokenを使用したあと、このメソッドはトークンを進め、parseExpressionをまた呼ぶ。このとき、前置演算子の優先順位を引数に渡す。
	p.nextToken()

	// この時点のトークンの位置は、1つ進んでいる。
	// -5の場合、 p.curToken.Type は token.INT 。この値をRightフィールドに設定して、返却
	expression.Right = p.parseExpression(PREFIX)

	return expression
}

// 中置演算式パース
// 引数としてleftという名前のast.Expressionを取ることに注意
func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	defer untrace(trace("parseInfixExpression"))
	expression := &ast.InfixExpression{
		Token:    p.curToken, // 現在のトークンは中置演算子式の演算子である
		Operator: p.curToken.Literal,
		Left:     left,
	}

	precedence := p.curPrecedence() // 優先順位保存
	p.nextToken()

	// この時点のトークンの位置は、1つ進んでいる
	expression.Right = p.parseExpression(precedence)

	return expression
}

// 括弧をパース
// 括られた式の優先順位が高まる
func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	exp := p.parseExpression(LOWEST)
	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return exp
}

// ifをパース
func (p *Parser) parseIfExpression() ast.Expression {
	expression := &ast.IfExpression{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	p.nextToken()
	expression.Condition = p.parseExpression(LOWEST)

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	expression.Consequence = p.parseBlockStatement()

	// elseの存在チェックし、存在する場合はelseの直後に来るブロック文を構文解析する
	// elseの省略は許す
	if p.peekTokenIs(token.ELSE) {
		p.nextToken()

		if !p.expectPeek(token.LBRACE) {
			return nil
		}

		expression.Alternative = p.parseBlockStatement()
	}

	return expression
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	block.Statements = []ast.Statement{}

	p.nextToken()

	for !p.curTokenIs(token.RBRACE) && !p.curTokenIs(token.EOF) {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}

// 関数リテラルをパース
func (p *Parser) parseFunctionLiteral() ast.Expression {
	lit := &ast.FunctionLiteral{Token: p.curToken}

	if !p.expectPeek(token.LPAREN) {
		return nil
	}

	lit.Parameters = p.parseFunctionParameters()

	if !p.expectPeek(token.LBRACE) {
		return nil
	}

	lit.Body = p.parseBlockStatement()

	return lit
}

// 引数をパース
func (p *Parser) parseFunctionParameters() []*ast.Identifier {
	identifiers := []*ast.Identifier{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return identifiers
	}

	p.nextToken()

	ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
	identifiers = append(identifiers, ident)

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		ident := &ast.Identifier{Token: p.curToken, Value: p.curToken.Literal}
		identifiers = append(identifiers, ident)
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return identifiers
}

// 関数呼び出しをパース
// すでに構文解析されたfunctionを引数として受け取り、ノードの構築に使う
func (p *Parser) parseCallExpression(function ast.Expression) ast.Expression {
	exp := &ast.CallExpression{Token: p.curToken, Function: function}
	exp.Arguments = p.parseCallArguments()
	return exp
}

// 関数呼び出しの引数をパース
func (p *Parser) parseCallArguments() []ast.Expression {
	args := []ast.Expression{}

	if p.peekTokenIs(token.RPAREN) {
		p.nextToken()
		return args
	}

	p.nextToken()
	args = append(args, p.parseExpression(LOWEST))

	for p.peekTokenIs(token.COMMA) {
		p.nextToken()
		p.nextToken()
		args = append(args, p.parseExpression(LOWEST))
	}

	if !p.expectPeek(token.RPAREN) {
		return nil
	}

	return args
}

// デバッグしやすいようにエラーメッセージを追加する
func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}

// 次のトークンタイプに対応している優先順位を返す
func (p *Parser) peekPrecedence() int {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}

	return LOWEST
}

// 現在のトークンタイプに対応している優先順位を返す
func (p *Parser) curPrecedence() int {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}

	return LOWEST
}
