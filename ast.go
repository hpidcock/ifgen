package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
)

// astExprForType returns the ast.Expr for the types.Type, adding any
// required imports.
func astExprForType(t types.Type, defaultImport string, imports map[string]int) ast.Expr {
	switch x := t.(type) {
	case *types.Named:
		switch x.Obj().Name() {
		case "error":
			return ast.NewIdent("error")
		case "compareable":
			return ast.NewIdent("compareable")
		case "any":
			// Might not be needed, does an any always come over as a types.Interface?
			return ast.NewIdent("any")
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
		case types.Int8:
			return &ast.Ident{Name: "int8"}
		case types.Int16:
			return &ast.Ident{Name: "int16"}
		case types.Int32:
			// TODO: figure out how to handle int32==rune
			return &ast.Ident{Name: "int32"}
		case types.Int64:
			return &ast.Ident{Name: "int64"}
		case types.Uint:
			return &ast.Ident{Name: "uint"}
		case types.Uint8:
			// TODO: figure out how to handle uint8==byte
			return &ast.Ident{Name: "uint8"}
		case types.Uint16:
			return &ast.Ident{Name: "uint16"}
		case types.Uint32:
			return &ast.Ident{Name: "uint32"}
		case types.Uint64:
			return &ast.Ident{Name: "uint64"}
		case types.Uintptr:
			return &ast.Ident{Name: "uintptr"}
		case types.Float32:
			return &ast.Ident{Name: "float32"}
		case types.Float64:
			return &ast.Ident{Name: "float64"}
		case types.Complex64:
			return &ast.Ident{Name: "complex64"}
		case types.Complex128:
			return &ast.Ident{Name: "complex128"}
		case types.String:
			return &ast.Ident{Name: "string"}
		case types.UnsafePointer:
			panic("unsafe pointer is not implemented")
		default:
			panic(fmt.Sprintf("unhandled basic kind %v", x.Kind()))
		}
	case *types.Interface:
		if x.Empty() {
			return ast.NewIdent("any")
		}
		out := &ast.InterfaceType{
			Methods: &ast.FieldList{},
		}
		for i := 0; i < x.NumMethods(); i++ {
			meth := x.Method(i)
			if !meth.Exported() {
				continue
			}

			sig := meth.Signature()
			funcType := &ast.FuncType{
				Params:  &ast.FieldList{},
				Results: &ast.FieldList{},
			}

			for j := 0; j < sig.Params().Len(); j++ {
				param := sig.Params().At(j)
				f := &ast.Field{}
				if param.Name() != "" {
					f.Names = append(f.Names, ast.NewIdent(param.Name()))
				}
				f.Type = astExprForType(param.Type(), defaultImport, imports)
				if sig.Variadic() && j == sig.Params().Len()-1 {
					f.Type = &ast.Ellipsis{
						Elt: f.Type.(*ast.ArrayType).Elt,
					}
				}
				funcType.Params.List = append(funcType.Params.List, f)
			}

			for j := 0; j < sig.Results().Len(); j++ {
				param := sig.Results().At(j)
				f := &ast.Field{}
				if param.Name() != "" {
					f.Names = append(f.Names, ast.NewIdent(param.Name()))
				}
				f.Type = astExprForType(param.Type(), defaultImport, imports)
				funcType.Results.List = append(funcType.Results.List, f)
			}
			field := &ast.Field{
				Names: []*ast.Ident{{
					Name: meth.Name(),
				}},
				Type: funcType,
			}
			out.Methods.List = append(out.Methods.List, field)
		}
		return out
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
		t := &ast.StructType{
			Fields: &ast.FieldList{},
		}
		for i := 0; i < x.NumFields(); i++ {
			f := x.Field(i)
			af := &ast.Field{
				Type: astExprForType(f.Type(), defaultImport, imports),
			}
			if f.Name() != "" {
				af.Names = append(af.Names, ast.NewIdent(f.Name()))
			}
			tag := x.Tag(i)
			if tag != "" {
				af.Tag = &ast.BasicLit{Kind: token.STRING, Value: tag}
			}
			t.Fields.List = append(t.Fields.List, af)
		}
		return t
	case *types.Pointer:
		return &ast.StarExpr{
			X: astExprForType(x.Elem(), defaultImport, imports),
		}
	default:
		panic("unhandled type " + reflect.TypeOf(t).String())
	}
}
