package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"

	"github.com/davecgh/go-spew/spew"
)

// astExprForType returns the ast.Expr for the types.Type, adding any
// required imports.
func astExprForType(t types.Type, defaultImport string, imports map[string]int) ast.Expr {
	switch x := t.(type) {
	case *types.Named:
		switch x.Obj().Name() {
		case "error":
			return ast.NewIdent("error")
		}
		pkg := defaultImport
		if x.Obj().Pkg() != nil {
			pkg = x.Obj().Pkg().Path()
		}
		imp, ok := imports[pkg]
		if !ok {
			next := -1
			for _, v := range imports {
				if v > next {
					next = v
				}
			}
			next = next + 1
			imports[pkg] = next
			imp = next
		}
		return ast.NewIdent(fmt.Sprintf("x%d.%s", imp, x.Obj().Name()))
	case *types.Array:
		return &ast.ArrayType{
			Elt: astExprForType(x.Elem(), defaultImport, imports),
			Len: &ast.BasicLit{Kind: token.INT, Value: strconv.Itoa(int(x.Len()))},
		}
	case *types.Slice:
		return &ast.ArrayType{
			Elt: astExprForType(x.Elem(), defaultImport, imports),
		}
	case *types.Map:
		return &ast.MapType{
			Key:   astExprForType(x.Key(), defaultImport, imports),
			Value: astExprForType(x.Elem(), defaultImport, imports),
		}
	case *types.Chan:
		var dir ast.ChanDir
		switch x.Dir() {
		case types.RecvOnly:
			dir = ast.RECV
		case types.SendOnly:
			dir = ast.SEND
		case types.SendRecv:
			dir = ast.RECV | ast.SEND
		default:
			panic("unknown channel direction")
		}
		return &ast.ChanType{
			Dir:   dir,
			Value: astExprForType(x.Elem(), defaultImport, imports),
		}
	case *types.Basic:
		switch x.Kind() {
		case types.Bool:
			return &ast.Ident{Name: "bool"}
		case types.Int:
			return &ast.Ident{Name: "int"}
		case types.Int64:
			return &ast.Ident{Name: "int64"}
		case types.String:
			return &ast.Ident{Name: "string"}
		default:
			panic(fmt.Sprintf("unhandled basic kind %v", x.Kind()))
		}
	case *types.Interface:
		if x.Empty() {
			return ast.NewIdent("any")
		}
		spew.Dump(x)
		panic("interface")
	case *types.Signature:
		t := &ast.FuncType{
			Params:  &ast.FieldList{},
			Results: &ast.FieldList{},
		}
		if p := x.Params(); p != nil {
			for i := 0; i < p.Len(); i++ {
				vr := p.At(i)
				f := &ast.Field{
					Type: astExprForType(vr.Type(), defaultImport, imports),
				}
				if x.Variadic() && i == p.Len()-1 {
					f.Type = &ast.Ellipsis{
						Elt: f.Type.(*ast.ArrayType).Elt,
					}
				}
				if vr.Name() != "" {
					f.Names = append(f.Names, ast.NewIdent(vr.Name()))
				}
				t.Params.List = append(t.Params.List, f)
			}
		}
		if p := x.Results(); p != nil {
			for i := 0; i < p.Len(); i++ {
				vr := p.At(i)
				f := &ast.Field{
					Type: astExprForType(vr.Type(), defaultImport, imports),
				}
				if vr.Name() != "" {
					f.Names = append(f.Names, ast.NewIdent(vr.Name()))
				}
				t.Results.List = append(t.Results.List, f)
			}
		}
		return t
	case *types.Struct:
		panic("struct")
	case *types.Pointer:
		return &ast.StarExpr{
			X: astExprForType(x.Elem(), defaultImport, imports),
		}
	default:
		panic("unhandled type " + reflect.TypeOf(t).String())
	}
}
