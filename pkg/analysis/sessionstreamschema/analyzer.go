package sessionstreamschema

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const Doc = "reject generic Struct top-level sessionstream schema payload registrations"

var Analyzer = &analysis.Analyzer{
	Name:     "sessionstreamschema",
	Doc:      Doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call := n.(*ast.CallExpr)
		if !isSchemaRegistrationCall(pass, call) || len(call.Args) < 2 {
			return
		}
		payload := call.Args[1]
		if isPointerToStructPBStruct(pass.TypesInfo.TypeOf(payload)) {
			pass.Reportf(payload.Pos(), "sessionstream schema registrations must use concrete protobuf messages, not *structpb.Struct")
		}
	})
	return nil, nil
}

func isSchemaRegistrationCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "RegisterCommand", "RegisterEvent", "RegisterUIEvent", "RegisterTimelineEntity":
		// continue
	default:
		return false
	}
	recv := pass.TypesInfo.TypeOf(sel.X)
	return isSessionstreamSchemaRegistry(recv)
}

func isSessionstreamSchemaRegistry(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Name() == "SchemaRegistry" && named.Obj().Pkg().Path() == "github.com/go-go-golems/sessionstream/pkg/sessionstream"
}

func isPointerToStructPBStruct(t types.Type) bool {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := ptr.Elem().(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Name() == "Struct" && named.Obj().Pkg().Path() == "google.golang.org/protobuf/types/known/structpb"
}
