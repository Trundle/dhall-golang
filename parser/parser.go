package parser

import (
	"strconv"

	"github.com/philandstuff/dhall-golang/ast"
	"github.com/prataprc/goparsec"
)

func unwrapOrdChoice(ns []parsec.ParsecNode) parsec.ParsecNode {
	if ns == nil || len(ns) < 1 {
		return nil
	}
	return ns[0]
}

func parseLineComment(ns []parsec.ParsecNode) parsec.ParsecNode {
	if ns == nil || len(ns) < 3 {
		return nil
	}
	return &Comment{Value: ns[1].(*parsec.Terminal).Value}
}

var LineComment = parsec.And(parseLineComment,
	parsec.AtomExact(`--`, "LINECOMMENTMARK"),
	parsec.TokenExact(`[\x20-\x{10ffff}\t]*`, "LINECOMMENT"),
	parsec.TokenExact(`\n|\r\n`, "EOL"),
)

type Comment struct {
	IsBlock bool
	Value   string
}

var WhitespaceChunk = parsec.OrdChoice(
	nil,
	parsec.TokenExact(`[ \t\n]|\r\n`, "WS"),
	LineComment,
	// BlockComment,
)

func parseWhitespace(ns []parsec.ParsecNode) parsec.ParsecNode {
	if ns == nil || len(ns) < 1 {
		return []Comment{}
	}
	var comments []Comment = []Comment{}
	for _, n := range ns {
		chunk := n.([]parsec.ParsecNode)[0]
		switch t := chunk.(type) {
		case *parsec.Terminal:
			continue
		case *Comment:
			comments = append(comments, *t)
		}
	}
	return comments
}

var Whitespace = parsec.Kleene(parseWhitespace, WhitespaceChunk)

func SkipWSAfter(parseFunc parsec.Nodify, p parsec.Parser) parsec.Parser {
	return parsec.And(parseFunc, p, Whitespace)
}

type AtomNode struct {
	Value    string
	Comments []Comment
}

func parseAtom(nodes []parsec.ParsecNode) parsec.ParsecNode {
	return &AtomNode{
		Value:    nodes[0].(*parsec.Terminal).Name,
		Comments: nodes[1].([]Comment),
	}
}

var Lambda = SkipWSAfter(parseAtom, parsec.TokenExact(`[λ\\]`, "LAMBDA"))
var OpenParens = SkipWSAfter(parseAtom, parsec.AtomExact(`(`, "OPAREN"))
var CloseParens = SkipWSAfter(parseAtom, parsec.AtomExact(`)`, "CPAREN"))
var Colon = SkipWSAfter(parseAtom, parsec.AtomExact(`:`, "COLON"))
var Arrow = SkipWSAfter(parseAtom, parsec.TokenExact(`(->|→)`, "ARROW"))
var TypeToken = SkipWSAfter(parseAtom, parsec.AtomExact(`Type`, "TYPE"))
var KindToken = SkipWSAfter(parseAtom, parsec.AtomExact(`Kind`, "KIND"))
var SortToken = SkipWSAfter(parseAtom, parsec.AtomExact(`Sort`, "SORT"))

var SimpleLabel = parsec.TokenExact(`[A-Za-z_][0-9a-zA-Z_/-]*`, "SIMPLE")

var Label = SkipWSAfter(parseLabel,
	SimpleLabel,
)

func parseConst(ns []parsec.ParsecNode) parsec.ParsecNode {

	switch n := ns[0].(type) {
	case *AtomNode:
		switch n.Value {
		case "TYPE":
			return ast.Type
		case "KIND":
			return ast.Kind
		case "SORT":
			return ast.Sort
		}
	}
	return nil
}

var Const = parsec.OrdChoice(parseConst, TypeToken, KindToken, SortToken)

func parseVar(ns []parsec.ParsecNode) parsec.ParsecNode {
	return ast.Var{Name: ns[0].(string)}
}

var Var = parsec.And(parseVar, Label)

func parseLabel(ns []parsec.ParsecNode) parsec.ParsecNode {
	if ns == nil || len(ns) < 1 {
		return nil
	}
	switch n := ns[0].(type) {
	case *parsec.Terminal:
		switch n.Name {
		case "SIMPLE":
			return n.Value
		}
	}
	return nil
}

func parseNatural(ns []parsec.ParsecNode) parsec.ParsecNode {
	if ns == nil || len(ns) < 1 {
		return nil
	}
	switch n := ns[0].(type) {
	case *parsec.Terminal:
		switch n.Name {
		case "DEC":
			val, _ := strconv.ParseUint(n.Value, 10, 64)
			return val
		case "OCT":
			val, _ := strconv.ParseUint(n.Value[1:], 8, 64)
			return val
		case "HEX":
			val, _ := strconv.ParseUint(n.Value[2:], 16, 64)
			return val
		}
	}
	return nil
}

var Natural = parsec.OrdChoice(parseNatural, parsec.Hex(), parsec.Oct(), parsec.Token(`[1-9][0-9]+`, "DEC"))
var Identifier = parsec.OrdChoice(nil, SimpleLabel)

func parseLambda(ns []parsec.ParsecNode) parsec.ParsecNode {
	label := ns[2].(string)
	t := ns[4]
	body := ns[7]
	return ast.NewLambdaExpr(label, t.(ast.Expr), body.(ast.Expr))
}

func expression(s parsec.Scanner) (parsec.ParsecNode, parsec.Scanner) {
	var Expr parsec.Parser = expression
	lambdaAbstraction := parsec.And(parseLambda,
		Lambda,
		OpenParens,
		Label,
		Colon,
		Expr,
		CloseParens,
		Arrow,
		Expr,
	)

	expr := parsec.OrdChoice(unwrapOrdChoice,
		lambdaAbstraction,
		Const,
		Var,
	)
	return expr(s)
}

var Expression parsec.Parser = expression

var CompleteExpression = parsec.And(nil, Whitespace, Expression)

// FIXME: if this doesn't parse all input, we have no way of knowing
func ParseExpression(input []byte) ast.Expr {
	expr, _ := Expression(parsec.NewScanner(input))
	return expr.(ast.Expr)
}
