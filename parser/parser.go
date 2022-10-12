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
	CALL        // myFunction(X)
)

// 字句解析器を受け取って初期化する
func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.prefixParseFns = make(map[token.TokenType]prefixParseFn)
	p.registerPrefix(token.IDENT, p.parseIdentifier) // もしトークンタイプtoken.IDENTが出現したら、呼び出すべき構文解析関数はparseIdentifier
	p.registerPrefix(token.INT, p.parseIntegerLiteral)
	p.registerPrefix(token.BANG, p.parsePrefixExpression)
	p.registerPrefix(token.MINUS, p.parsePrefixExpression)

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

// パースを開始。トークンを1つずつ辿る
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
	stmt := &ast.ExpressionStatement{Token: p.curToken}

	stmt.Expression = p.parseExpression(LOWEST)

	// セミコロンは省略可能。あとでREPLに入力しやすくなる
	if p.peekTokenIs(token.SEMICOLON) {
		p.nextToken()
	}

	return stmt
}

// p.curToken.Typeの前置に関連付けられた構文解析関数があるかを確認し、存在していればその構文解析関数を呼び出し、その結果を返す
func (p *Parser) parseExpression(precedence int) ast.Expression {
	prefix := p.prefixParseFns[p.curToken.Type]
	if prefix == nil {
		p.noPrefixParseFnError(p.curToken.Type)
		return nil
	}
	leftExp := prefix()

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

// デバッグしやすいようにエラーメッセージを追加する
func (p *Parser) noPrefixParseFnError(t token.TokenType) {
	msg := fmt.Sprintf("no prefix parse function for %s found", t)
	p.errors = append(p.errors, msg)
}
