package evaluator

import (
	"fmt"
	"monkey/ast"
	"monkey/object"
	"monkey/token"
)

func quote(node ast.Node, env *object.Environment) object.Object {
	node = evalUnquoteCalls(node, env)
	return &object.Quote{Node: node}
}

func evalUnquoteCalls(quoted ast.Node, env *object.Environment) ast.Node {
	return ast.Modify(quoted, func(node ast.Node) ast.Node {
		// 呼び出しがunquoteではなかったら何もしない
		if !isUnquoteCall(node) {
			return node
		}

		call, ok := node.(*ast.CallExpression)
		if !ok {
			return node
		}
		if len(call.Arguments) != 1 {
			return node
		}

		unquoted := Eval(call.Arguments[0], env)
		// unquoteの呼び出しを置換し、結果を逆に未評価のast.Nodeに挿入する。そのためにEvalした結果(object.Object)をast.Nodeに変換する
		return convertObjectToASTNode(unquoted)
	})
}

func isUnquoteCall(node ast.Node) bool {
	callExpression, ok := node.(*ast.CallExpression)
	if !ok {
		return false
	}

	return callExpression.Function.TokenLiteral() == "unquote"
}

func convertObjectToASTNode(obj object.Object) ast.Node {
	switch obj := obj.(type) {
	case *object.Integer:
		t := token.Token{
			Type:    token.INT,
			Literal: fmt.Sprintf("%d", obj.Value),
		}
		return &ast.IntegerLiteral{Token: t, Value: obj.Value}
	case *object.Boolean:
		var t token.Token
		if obj.Value {
			t = token.Token{Type: token.TRUE, Literal: "true"}
		} else {
			t = token.Token{Type: token.FALSE, Literal: "false"}
		}
		return &ast.Boolean{Token: t, Value: obj.Value}
	case *object.Quote:
		return obj.Node
	default:
		return nil
	}
}
