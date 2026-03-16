package calculator

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"

	"github.com/lioarce01/chainforge/pkg/core"
	"github.com/lioarce01/chainforge/pkg/tools"
)

// Calculator is a safe math expression evaluator using Go's AST parser.
// It supports +, -, *, /, %, ^ (power), parentheses, and standard math functions.
type Calculator struct {
	def core.ToolDefinition
}

// New creates a new Calculator tool.
func New() *Calculator {
	schema := tools.NewSchema().
		Add("expression", tools.Property{
			Type:        tools.TypeString,
			Description: "Mathematical expression to evaluate, e.g. '2^10 + 144' or 'sqrt(16) * 3'",
		}, true).
		MustBuild()

	return &Calculator{
		def: core.ToolDefinition{
			Name:        "calculator",
			Description: "Evaluates mathematical expressions safely. Supports +, -, *, /, ^ (power), sqrt, abs, floor, ceil, round, sin, cos, tan, log, log2, pi, e.",
			InputSchema: schema,
		},
	}
}

func (c *Calculator) Definition() core.ToolDefinition { return c.def }

func (c *Calculator) Call(_ context.Context, input string) (string, error) {
	var req struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", fmt.Errorf("calculator: invalid input: %w", err)
	}
	if req.Expression == "" {
		return "", fmt.Errorf("calculator: expression is required")
	}

	result, err := eval(req.Expression)
	if err != nil {
		return "", fmt.Errorf("calculator: %w", err)
	}

	// Format: integer if possible, otherwise float with up to 10 decimal places
	if result == math.Trunc(result) && !math.IsInf(result, 0) {
		return strconv.FormatInt(int64(result), 10), nil
	}
	return strconv.FormatFloat(result, 'f', 10, 64), nil
}

// eval evaluates a math expression string.
func eval(expr string) (float64, error) {
	// Replace ^ with ** for Go parser compatibility — we handle it in evalNode
	// Go parser doesn't support **, so we handle ^ as XOR token and treat it as power
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("parse error: %w", err)
	}
	return evalNode(node)
}

func evalNode(node ast.Expr) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		switch n.Kind {
		case token.INT:
			v, err := strconv.ParseInt(n.Value, 0, 64)
			return float64(v), err
		case token.FLOAT:
			return strconv.ParseFloat(n.Value, 64)
		}
		return 0, fmt.Errorf("unsupported literal: %v", n.Value)

	case *ast.BinaryExpr:
		x, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		y, err := evalNode(n.Y)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.ADD:
			return x + y, nil
		case token.SUB:
			return x - y, nil
		case token.MUL:
			return x * y, nil
		case token.QUO:
			if y == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return x / y, nil
		case token.REM:
			if y == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			return math.Mod(x, y), nil
		case token.XOR: // ^ used as power operator
			return math.Pow(x, y), nil
		}
		return 0, fmt.Errorf("unsupported operator: %v", n.Op)

	case *ast.UnaryExpr:
		x, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.SUB:
			return -x, nil
		case token.ADD:
			return x, nil
		}
		return 0, fmt.Errorf("unsupported unary operator: %v", n.Op)

	case *ast.ParenExpr:
		return evalNode(n.X)

	case *ast.CallExpr:
		return evalFunc(n)

	case *ast.Ident:
		switch n.Name {
		case "pi", "Pi":
			return math.Pi, nil
		case "e", "E":
			return math.E, nil
		}
		return 0, fmt.Errorf("unknown identifier: %q", n.Name)
	}

	return 0, fmt.Errorf("unsupported expression type: %T", node)
}

func evalFunc(call *ast.CallExpr) (float64, error) {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return 0, fmt.Errorf("unsupported function call type")
	}

	args := make([]float64, len(call.Args))
	for i, arg := range call.Args {
		v, err := evalNode(arg)
		if err != nil {
			return 0, err
		}
		args[i] = v
	}

	oneArg := func() (float64, error) {
		if len(args) != 1 {
			return 0, fmt.Errorf("%s requires exactly 1 argument", ident.Name)
		}
		return args[0], nil
	}

	switch ident.Name {
	case "sqrt":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Sqrt(x), nil
	case "abs":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Abs(x), nil
	case "floor":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Floor(x), nil
	case "ceil":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Ceil(x), nil
	case "round":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Round(x), nil
	case "sin":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Sin(x), nil
	case "cos":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Cos(x), nil
	case "tan":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Tan(x), nil
	case "log":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Log(x), nil
	case "log2":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Log2(x), nil
	case "log10":
		x, err := oneArg()
		if err != nil {
			return 0, err
		}
		return math.Log10(x), nil
	case "pow":
		if len(args) != 2 {
			return 0, fmt.Errorf("pow requires exactly 2 arguments")
		}
		return math.Pow(args[0], args[1]), nil
	case "max":
		if len(args) != 2 {
			return 0, fmt.Errorf("max requires exactly 2 arguments")
		}
		return math.Max(args[0], args[1]), nil
	case "min":
		if len(args) != 2 {
			return 0, fmt.Errorf("min requires exactly 2 arguments")
		}
		return math.Min(args[0], args[1]), nil
	}

	return 0, fmt.Errorf("unknown function: %q", ident.Name)
}
