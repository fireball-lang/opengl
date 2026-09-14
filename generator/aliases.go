package main

import (
	"cmp"
	"slices"
	"strings"

	"github.com/fireball-lang/bindgen"
	"github.com/fireball-lang/bindgen/fb"
)

func GetFbFuncAliases(enums map[string]*fb.Enum, active map[string]Command) []*fb.Alias {
	customTypeParser := MapCustomType(enums)

	getEnumOrU32 := func(group string) fb.Type {
		if enum, ok := enums[group]; ok {
			return &fb.DeclType{Decl: enum}
		}

		return &fb.SimpleType{Text: "u32"}
	}

	parseType := func(str, group string) fb.Type {
		typ := bindgen.ParseCType(str, customTypeParser)

		if group != "" {
			if enum, ok := enums[group]; ok {
				switch t := typ.(type) {
				case *fb.SimpleType:
					typ = &fb.DeclType{Decl: enum}

				case *fb.PointerType:
					curr := t

					for {
						if next, ok := curr.Pointee.(*fb.PointerType); ok {
							curr = next
						} else {
							curr.Pointee = &fb.DeclType{Decl: enum}
							break
						}
					}
				}
			}
		}

		return typ
	}

	aliases := make([]*fb.Alias, 0, 1+len(active))

	aliases = append(aliases, &fb.Alias{
		OutputIndex: 1,
		Name:        "GlDebugProc",
		Type: &fb.FuncType{
			Params: []*fb.Param{
				{Name: "source", Type: getEnumOrU32("DebugSource")},
				{Name: "gl_type", Type: getEnumOrU32("DebugType")},
				{Name: "id", Type: &fb.SimpleType{Text: "u32"}},
				{Name: "severity", Type: getEnumOrU32("DebugSeverity")},
				{Name: "length", Type: &fb.SimpleType{Text: "i32"}},
				{Name: "message", Type: &fb.PointerType{Pointee: &fb.SimpleType{Text: "u8"}}},
				{Name: "user_param", Type: &fb.PointerType{Pointee: &fb.SimpleType{Text: "void"}}},
			},
			Returns: &fb.SimpleType{Text: "void"},
		},
	})

	for _, command := range active {
		params := make([]*fb.Param, len(command.Params))

		for i, param := range command.Params {
			name := bindgen.CamelToSnakeCase(param.Name)
			if slices.Contains(bindgen.KEYWORDS, name) {
				name += "_"
			}

			params[i] = &fb.Param{
				Name: name,
				Type: parseType(param.Type, param.Group),
			}
		}

		name := command.Proto.Name
		name = strings.ToUpper(name[:1]) + name[1:]

		aliases = append(aliases, &fb.Alias{
			OutputIndex: 1,
			Name:        name,
			Type: &fb.FuncType{
				Params:  params,
				Returns: parseType(command.Proto.Returns, command.Proto.Group),
			},
		})
	}

	slices.SortFunc(aliases, func(a, b *fb.Alias) int {
		if a.Name == "GlDebugProc" {
			return -1
		}
		if b.Name == "GlDebugProc" {
			return 1
		}

		return cmp.Compare(a.Name, b.Name)
	})

	return aliases
}

func WriteAliases(w fb.Writer, aliases []*fb.Alias) {
	for _, alias := range aliases {
		w.Write("\n")
		alias.Write(w)
	}
}

func WriteStubs(w fb.Writer, aliases []*fb.Alias) {
	for _, alias := range aliases {
		if alias.Name == "GlDebugProc" {
			continue
		}

		w.Write("\n")

		// Signature
		typ := alias.Type.(*fb.FuncType)
		name := "stub_" + bindgen.CamelToSnakeCase(strings.TrimPrefix(alias.Name, "Gl"))

		params := make([]*fb.Param, len(typ.Params))

		for i, param := range typ.Params {
			params[i] = &fb.Param{
				Name: "_" + param.Name,
				Type: param.Type,
			}
		}

		w.Write("pub ")
		returns := fb.WriteSignature(w, name, fb.None, params, typ.Returns)

		// Body
		w.Write(" {\n    panic(\"OpenGL function not found\");\n")

		if returns {
			switch typ := typ.Returns.(type) {
			case *fb.SimpleType:
				if typ.Text == "bool" {
					w.Write("    return false;\n")
				} else {
					w.Write("    return 0;\n")
				}

			case *fb.PointerType:
				w.Write("    return null;\n")

			case *fb.DeclType:
				w.Write("    return 0 as ")
				typ.Write(w)
				w.Write(";\n")
			}
		}

		w.Write("}\n")
	}
}
