// This program looks for the //templ:component-opts directive on any struct in a directory tree
// and generates a series of functions and methods to help use that struct in a templ component.

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const (
	// CodegenDirective is the comment that will trigger code generation for a specific struct
	CodegenDirective = "//templ:component-opts"

	// CodegenSuffix is the suffix for files generated by this program
	CodegenSuffix = "_tcogen.go"

	// TemplSuffix is the suffix for files generated by templ
	TemplSuffix = "_templ.go"
)

var (
	optTypeName = ast.NewIdent("Opt")
)

func main() {
	root, _ := os.Getwd()
	// Get the root directory as the first argument
	if len(os.Args) >= 2 {
		// // fmt.Printf("Usage: %s <root_dir>\n", os.Args[0])
		// os.Exit(1)
		root = os.Args[1]
	}

	// Walk the directory tree and parse the Go files
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, non-Go, and Go files generated by this tool
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, CodegenSuffix) || strings.HasSuffix(path, TemplSuffix) {
			return nil
		}

		// Parse the Go file
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}

		// Find the structs that have the directive
		findStructs(path, fset, file)

		return nil
	})

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// findStructs finds the structs that have the magic directive in a given file
func findStructs(path string, fset *token.FileSet, file *ast.File) {
	var genDecls []*ast.GenDecl

	for node, commentGroup := range ast.NewCommentMap(fset, file, file.Comments) {
		for _, comments := range commentGroup {
			for _, comment := range comments.List {
				// search for comments containing our directive
				if comment.Text != CodegenDirective {
					continue
				}

				// Check if the declaration is a type declaration
				if genDecl, ok := node.(*ast.GenDecl); ok {
					genDecls = append(genDecls, genDecl)
				}
			}
		}
	}

	// Iterate over the declarations in the file
	for _, genDecl := range genDecls {
		// Iterate over the specs in the declaration
		for _, spec := range genDecl.Specs {
			// Check if the spec is a type spec
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Check if the type is a struct type
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			structName := typeSpec.Name.Name
			if structType.Fields == nil || len(structType.Fields.List) == 0 {
				fmt.Println(structName + "has no fields; skipping")
				continue
			}

			genFuncs(path, fset, file, structName, structType.Fields.List)
		}
	}
}

// genFuncs generates functions for each field in the struct
func genFuncs(path string, fset *token.FileSet, file *ast.File, structName string, fields []*ast.Field) {

	// Create a new file to hold the generated functions
	newPath := strings.Replace(path, ".go", CodegenSuffix, 1)
	fmt.Printf("Found %s.%s; generating %s...\n", file.Name.Name, structName, newPath)
	out, err := os.Create(newPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer out.Close()

	newFile := &ast.File{
		Comments: []*ast.CommentGroup{
			{
				List: []*ast.Comment{
					{
						Text: "// Code generated by templ-component-opts; DO NOT EDIT.\n\n",
					},
					{
						Text: "// This file contains functions and methods for use with " + structName + " in templ components.\n\n",
					},
				},
			},
		},
		Package: 2,
		Name:    ast.NewIdent(file.Name.Name),
	}

	// include all imports from the source file
	imports := &ast.GenDecl{
		TokPos: newFile.Package,
		Tok:    token.IMPORT,
		Specs:  make([]ast.Spec, 0),
	}
	imports.Specs = append(imports.Specs, &ast.ImportSpec{
		Path: &ast.BasicLit{Value: `"strconv"`},
	})
	for _, srcImport := range file.Imports {
		imports.Specs = append(imports.Specs, &ast.ImportSpec{
			Path: &ast.BasicLit{Value: srcImport.Path.Value},
		})
	}
	newFile.Decls = append(newFile.Decls, imports)

	structNameIdent := ast.NewIdent(structName)
	genPrelude(fset, newFile, structNameIdent, fields)

	// Iterate over the fields
	for _, field := range fields {
		// Check if the field has a name
		if len(field.Names) == 0 {
			continue
		}

		// Get the field name and type
		fieldName := field.Names[0].Name
		fieldType := field.Type

		// Generate a function for the field
		genFunc(fset, newFile, structNameIdent, fieldName, fieldType)
	}

	// Write the new file to genFile
	printer.Fprint(out, fset, newFile)
}

func genPrelude(fset *token.FileSet, file *ast.File, structName *ast.Ident, fields []*ast.Field) {
	// define a new Opt type that is simply a function which takes a pointer to the struct
	optType := &ast.TypeSpec{
		Name: optTypeName,
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						// input parameter that is a pointer to the struct
						Type: &ast.StarExpr{
							X: structName,
						},
					},
				},
			},
		},
	}

	// declare the new Opt type
	file.Decls = append(file.Decls, &ast.GenDecl{
		Tok:   token.TYPE,
		Specs: []ast.Spec{optType},
	})

	genDefaultFunc(fset, file, structName, fields)
	genWithFunc(fset, file, structName)
	genWithMethod(fset, file, structName)
}

// genDefaultFunc generates a function that returns a new struct with default values.
func genDefaultFunc(fset *token.FileSet, file *ast.File, structName *ast.Ident, fields []*ast.Field) {
	// find the value for fields with a "default" tag
	var fieldDefaults []ast.Expr
	for _, field := range fields {
		fieldName := field.Names[0].Name
		if field.Tag == nil {
			continue
		}

		tagValue := field.Tag.Value
		tag := reflect.StructTag(tagValue[1 : len(tagValue)-1]) // remove quotes

		defValue := tag.Get("default")
		fieldIdent, ok := field.Type.(*ast.Ident)
		if !ok {
			continue
		}

		switch fieldIdent.Name {
		case "string":
			// strings need to be quoted
			defValue = fmt.Sprintf("%q", defValue)
		default:
			// TODO: handle other types?
		}

		fieldDefaults = append(fieldDefaults, &ast.KeyValueExpr{
			Key: ast.NewIdent(fieldName),
			Value: &ast.BasicLit{
				Kind:  field.Tag.Kind,
				Value: defValue,
			},
		})
	}

	decl := &ast.FuncDecl{
		// define the function name
		Name: ast.NewIdent("DefaultOpts"),

		// define the method signature
		Type: &ast.FuncType{
			// return a pointer to the same struct
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						// return a pointer to the struct
						Type: &ast.StarExpr{
							X: structName,
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("out")},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.UnaryExpr{
							Op: token.AND,
							X: &ast.CompositeLit{
								Type: structName,
								Elts: fieldDefaults,
							},
						},
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("out")},
				},
			},
		},
	}

	file.Decls = append(file.Decls, decl)
}

// genWithFunc generates a function that is used to apply options to the struct defaults.
func genWithFunc(fset *token.FileSet, file *ast.File, structName *ast.Ident) {
	// generate a new function to build a struct with a series of options
	withDecl := &ast.FuncDecl{
		Name: ast.NewIdent("With"),
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						// take one or more Opts
						Names: []*ast.Ident{ast.NewIdent("opts")},
						Type: &ast.Ellipsis{
							Elt: optTypeName,
						},
					},
				},
			},

			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						// return a pointer to the struct
						Type: &ast.StarExpr{
							X: structName,
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("out")},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.CallExpr{
							Fun: ast.NewIdent("DefaultOpts"),
						},
					},
				},
				&ast.ExprStmt{
					// Create the call expression node
					X: &ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   ast.NewIdent("out"),
							Sel: ast.NewIdent("With"),
						},
						Args: []ast.Expr{
							ast.NewIdent("opts"),
						},
						Ellipsis: 1,
					},
				},
				&ast.ReturnStmt{
					Results: []ast.Expr{ast.NewIdent("out")},
				},
			},
		},
	}

	file.Decls = append(file.Decls, withDecl)
}

// genWithMethod generates a method on the target struct that is used to apply options to the struct.
func genWithMethod(fset *token.FileSet, file *ast.File, structName *ast.Ident) {
	selfIdent := ast.NewIdent("o")
	optsParam := ast.NewIdent("opts")
	optVar := ast.NewIdent("opt")

	withDecl := &ast.FuncDecl{
		// define the method receiver
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{selfIdent},
					Type: &ast.StarExpr{
						X: structName,
					},
				},
			},
		},

		// define the method name
		Name: ast.NewIdent("With"),

		// define the method signature
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						// receive one or more Opts
						Names: []*ast.Ident{optsParam},
						Type: &ast.Ellipsis{
							Elt: optTypeName,
						},
					},
				},
			},

			// return a pointer to the same struct
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: &ast.StarExpr{
							X: structName,
						},
					},
				},
			},
		},

		// define the method body
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// iterate over the Opts
				&ast.RangeStmt{
					Key:   ast.NewIdent("_"),
					Value: optVar,
					Tok:   token.DEFINE,
					X:     optsParam,
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							// Create the expression statement node
							&ast.ExprStmt{
								// Create the call expression node
								X: &ast.CallExpr{
									Fun: optVar,
									Args: []ast.Expr{
										selfIdent,
									},
								},
							},
						},
					},
				},

				// return the struct for easy chaining
				&ast.ReturnStmt{
					Results: []ast.Expr{selfIdent},
				},
			},
		},
	}

	file.Decls = append(file.Decls, withDecl)
}

// genFunc generates a package-level function for a given field in the specified struct.
func genFunc(fset *token.FileSet, file *ast.File, structName *ast.Ident, fieldName string, fieldType ast.Expr) {
	inIdent := ast.NewIdent("in")
	fieldIdent := ast.NewIdent(fieldName)

	// Create the function signature
	funcSig := &ast.FuncType{
		Params: &ast.FieldList{
			List: []*ast.Field{
				{
					// receive an input parameter called "in"
					Names: []*ast.Ident{inIdent},
					Type:  fieldType,
				},
			},
		},
		Results: &ast.FieldList{
			List: []*ast.Field{
				{
					// return an Opt
					Type: optTypeName,
				},
			},
		},
	}

	// Create the Opt to set the value for a field
	closure := &ast.FuncLit{
		Type: &ast.FuncType{
			Params: &ast.FieldList{
				List: []*ast.Field{
					{
						Names: []*ast.Ident{ast.NewIdent("opts")},
						Type: &ast.StarExpr{
							X: structName,
						},
					},
				},
			},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				// Assign a value to the specified field
				&ast.AssignStmt{
					Lhs: []ast.Expr{ast.NewIdent("opts." + fieldName)},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{inIdent},
				},
			},
		},
	}

	// Create the function declaration
	funcDecl := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{
					// TODO: figure out why this doesn't work
					Text: fmt.Sprintf("// %s sets the value of the %s.%s field\n", fieldName, structName, fieldName),
				},
			},
		},
		Name: fieldIdent,
		Type: funcSig,
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.ReturnStmt{
					Results: []ast.Expr{closure},
				},
			},
		},
	}

	// Write the function declaration to the file
	file.Decls = append(file.Decls, funcDecl)

	// some data types need an additional method to return the value as a string, which is required inside templ components
	if fieldTypeIdent, ok := fieldType.(*ast.Ident); ok {
		if fieldTypeIdent.Name != "string" {
			genStrFunc(fset, file, structName, fieldName, fieldType, fieldTypeIdent)
		}
	}
}

// genStrFunc generates a helper method to return the value of a specific field as a string
func genStrFunc(fset *token.FileSet, file *ast.File, structName *ast.Ident, fieldName string, fieldType ast.Expr, fieldTypeIdent *ast.Ident) {
	selfIdent := ast.NewIdent("o")
	fieldIdent := ast.NewIdent(fieldName)

	fnDecl := &ast.FuncDecl{
		Doc: &ast.CommentGroup{
			List: []*ast.Comment{
				{
					// TODO: figure out why this doesn't work
					Text: fmt.Sprintf("// %s returns a string form of the %s.%s field\n", fieldName, structName, fieldName),
				},
			},
		},
		// define the method receiver
		Recv: &ast.FieldList{
			List: []*ast.Field{
				{
					Names: []*ast.Ident{selfIdent},
					Type: &ast.StarExpr{
						X: structName,
					},
				},
			},
		},

		// define the method name
		Name: ast.NewIdent(fieldName + "Str"),

		// define the method signature
		Type: &ast.FuncType{
			// return a string
			Results: &ast.FieldList{
				List: []*ast.Field{
					{
						Type: ast.NewIdent("string"),
					},
				},
			},
		},

		// define the method body
		Body: &ast.BlockStmt{
			List: []ast.Stmt{},
		},
	}

	switch fieldTypeIdent.Name {
	case "bool":
		fnDecl.Body.List = append(fnDecl.Body.List,
			&ast.ReturnStmt{
				Results: []ast.Expr{
					&ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   ast.NewIdent("strconv"),
							Sel: ast.NewIdent("FormatBool"),
						},
						Args: []ast.Expr{
							&ast.SelectorExpr{
								X:   selfIdent,
								Sel: fieldIdent,
							},
						},
					},
				},
			},
		)
	case "float64":
		fnDecl.Body.List = append(fnDecl.Body.List,
			&ast.ReturnStmt{
				Results: []ast.Expr{
					&ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   ast.NewIdent("strconv"),
							Sel: ast.NewIdent("FormatFloat"),
						},
						Args: []ast.Expr{
							&ast.SelectorExpr{
								X:   selfIdent,
								Sel: fieldIdent,
							},
							&ast.BasicLit{
								Kind:  token.CHAR,
								Value: "'f'",
							},
							&ast.BasicLit{
								Kind:  token.INT,
								Value: "1",
							},
							&ast.BasicLit{
								Kind:  token.INT,
								Value: "64",
							},
						},
					},
				},
			},
		)
	case "int64":
		fnDecl.Body.List = append(fnDecl.Body.List,
			&ast.ReturnStmt{
				Results: []ast.Expr{
					&ast.CallExpr{
						Fun: &ast.SelectorExpr{
							X:   ast.NewIdent("strconv"),
							Sel: ast.NewIdent("FormatInt"),
						},
						Args: []ast.Expr{
							&ast.SelectorExpr{
								X:   selfIdent,
								Sel: fieldIdent,
							},
							&ast.BasicLit{
								Kind:  token.INT,
								Value: "10",
							},
						},
					},
				},
			},
		)
	}

	file.Decls = append(file.Decls, fnDecl)
}
