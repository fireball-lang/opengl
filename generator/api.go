package main

import (
	"strings"

	"github.com/fireball-lang/bindgen"
	"github.com/fireball-lang/bindgen/fb"
)

func WriteApi(w fb.Writer, aliases []*fb.Alias, extensions []string) {
	// malloc
	w.Write("\n")
	w.Write("#[extern]\n")
	w.Write("func malloc(size: u64) mut *void;\n")

	// Api
	fields := make([]*fb.Field, 0, 2+len(aliases)-1)

	fields = append(fields, &fb.Field{
		Public: true,
		Name:   "version",
		Type:   &fb.SimpleType{Text: "opengl::Version"},
	})

	fields = append(fields, &fb.Field{
		Public: true,
		Name:   "extensions",
		Type: &fb.ArrayType{
			Size:    uint32(len(extensions)),
			Element: &fb.SimpleType{Text: "bool"},
		},
	})

	for _, alias := range aliases {
		if alias.Name == "GlDebugProc" {
			continue
		}

		name := bindgen.CamelToSnakeCase(strings.TrimPrefix(alias.Name, "Gl"))

		fields = append(fields, &fb.Field{
			Public: true,
			Name:   "ref_" + name,
			Type:   &fb.DeclType{Decl: alias},
		})
	}

	api := &fb.Struct{
		OutputIndex: 3,
		Name:        "Api",
		Fields:      fields,
	}

	w.Write("\n")
	api.Write(w)
}

func WriteApiLoad(w fb.Writer, versions []Version, extensions []string, extensionCommands map[string][]string) {
	w.Write("\n")
	w.Write("impl Api {\n")
	w.Write("    pub func load(fn: func(name: StringView) *void) ?&Api {\n")
	w.Write("        var api = malloc(sizeof(Self)) as mut &Self;\n")
	w.Write("        api.init_stubs();\n")
	w.Write("        var ptr: *void;\n")

	// Bootstrap essential functions
	w.Write("\n")
	w.Write("        ptr = fn(\"glGetIntegerv\");\n")
	w.Write("        if (ptr == null) return Option:[&Self]::none();\n")
	w.Write("        api.ref_get_integerv = ptr as opengl::GlGetIntegerv;\n")

	w.Write("\n")
	w.Write("        ptr = fn(\"glGetStringi\");\n")
	w.Write("        if (ptr == null) return Option:[&Self]::none();\n")
	w.Write("        api.ref_get_stringi = ptr as opengl::GlGetStringi;\n")

	// Query Version
	w.Write("\n")
	w.Write("        var major: i32;\n")
	w.Write("        api.get_integerv(opengl::GetPName::MajorVersion, &major);\n")
	w.Write("        if (major < 1 || major > 255) return Option:[&Self]::none();\n")
	w.Write("        api.version.major = major as u8;\n")

	w.Write("\n")
	w.Write("        var minor: i32;\n")
	w.Write("        api.get_integerv(opengl::GetPName::MinorVersion, &minor);\n")
	w.Write("        if (minor < 0 || minor > 255) return Option:[&Self]::none();\n")
	w.Write("        api.version.minor = minor as u8;\n")

	// Load Versions
	w.Write("\n")

	for _, version := range versions {
		w.Write("        if (opengl::Version::new(%d, %d) <= api.version) api.load_%d_%d(fn);\n", version.Major, version.Minor, version.Major, version.Minor)
	}

	// Load Extensions
	w.Write("\n")
	w.Write("        var num_extensions: i32;\n")
	w.Write("        api.get_integerv(opengl::GetPName::NumExtensions, &num_extensions);\n")

	for i, ext := range extensions {
		w.Write("\n")
		w.Write("        api.extensions[%d] = api.check_extension(num_extensions, \"%s\".ptr);\n", i, ext)

		if len(extensionCommands[ext]) != 0 {
			w.Write("        if (api.extensions[%d]) api.load_%s(fn);\n", i, ext)
		}
	}

	w.Write("\n        return api;\n")
	w.Write("    }\n")
	w.Write("}\n")
}

func WriteApiInitStubs(w fb.Writer, aliases []*fb.Alias) {
	w.Write("\n")
	w.Write("impl Api {\n")
	w.Write("    func init_stubs(mut self) {\n")

	for _, alias := range aliases {
		if alias.Name == "GlDebugProc" {
			continue
		}

		name := bindgen.CamelToSnakeCase(strings.TrimPrefix(alias.Name, "Gl"))
		w.Write("        self.ref_%s = opengl::stub_%s;\n", name, name)
	}

	w.Write("    }\n")
	w.Write("}\n")
}

func WriteApiLoadVersion(w fb.Writer, version Version, commands []string) {
	w.Write("\n")
	w.Write("impl opengl::Api {\n")
	w.Write("    func load_%d_%d(mut self, fn: func(name: StringView) *void) ?bool {\n", version.Major, version.Minor)
	w.Write("        var ptr: *void;\n")

	for _, command := range commands {
		WriteLoadFunc(w, "self", command)
	}

	w.Write("\n")
	w.Write("        return true;\n")

	w.Write("    }\n")
	w.Write("}\n")
}

func WriteApiLoadExtension(w fb.Writer, ext string, commands []string) {
	w.Write("\n")
	w.Write("impl opengl::Api {\n")
	w.Write("    func load_%s(mut self, fn: func(name: StringView) *void) ?bool {\n", ext)
	w.Write("        var ptr: *void;\n")

	for _, command := range commands {
		WriteLoadFunc(w, "self", command)
	}

	w.Write("\n")
	w.Write("        return true;\n")

	w.Write("    }\n")
	w.Write("}\n")
}

func WriteApiMethodWrappers(w fb.Writer, aliases []*fb.Alias) {
	w.Write("\n")
	w.Write("impl opengl::Api {\n")

	i := 0

	for _, alias := range aliases {
		if alias.Name == "GlDebugProc" {
			continue
		}

		if i > 0 {
			w.Write("\n")
		}

		i++

		// Signature
		typ := alias.Type.(*fb.FuncType)
		name := bindgen.CamelToSnakeCase(strings.TrimPrefix(alias.Name, "Gl"))

		w.Write("    pub ")
		returns := fb.WriteSignature(w, name, fb.Immutable, typ.Params, typ.Returns)

		// Body
		w.Write(" {\n")

		if returns {
			w.Write("        return ")
		} else {
			w.Write("        ")
		}

		w.Write("self.ref_%s(", name)

		for i, param := range typ.Params {
			if i > 0 {
				w.Write(", ")
			}

			w.Write(param.Name)
		}

		w.Write(");\n")
		w.Write("    }\n")
	}

	w.Write("}\n")
}

func WriteLoadFunc(w fb.Writer, self, name string) {
	aliasName := strings.ToUpper(name[:1]) + name[1:]
	funcName := bindgen.CamelToSnakeCase(strings.TrimPrefix(aliasName, "Gl"))

	w.Write("\n")
	w.Write("        ptr = fn(\"%s\");\n", name)
	w.Write("        if (ptr != null) %s.ref_%s = ptr as opengl::%s;\n", self, funcName, aliasName)
}
