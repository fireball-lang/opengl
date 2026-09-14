package main

import (
	"slices"
	"strconv"
	"strings"

	"github.com/fireball-lang/bindgen"
	"github.com/fireball-lang/bindgen/fb"
)

func main() {
	reg := LoadRegistry()

	versions := reg.GetVersions()
	activeEnums, activeCommands, versionCommands, activeExtensions, extensionCommands := reg.GetActiveItems(versions)

	enums := GetFbEnums(reg, activeEnums)
	aliases := GetFbFuncAliases(enums, activeCommands)

	// Outputs
	outputs := []bindgen.File{
		{Path: "../src/enums.fb", Module: "opengl"},
		{Path: "../src/aliases.fb", Module: "opengl"},
		{Path: "../src/stubs.fb", Module: "opengl"},
		{Path: "../src/api.fb", Module: "opengl"},
		{Path: "../src/api_load_versions.fb", Module: "opengl"},
		{Path: "../src/api_load_extensions.fb", Module: "opengl"},
		{Path: "../src/api_load_wrappers.fb", Module: "opengl"},
	}

	// Enums
	{
		w, err := bindgen.NewFileWriter(outputs, 0)
		if err != nil {
			panic(err.Error())
		}

		WriteExtensionEnum(w, activeExtensions)
		WriteEnums(w, enums)
		_ = w.Close()
	}

	// Aliases
	{
		w, err := bindgen.NewFileWriter(outputs, 1)
		if err != nil {
			panic(err.Error())
		}

		WriteAliases(w, aliases)
		_ = w.Close()
	}

	// Stubs
	{
		w, err := bindgen.NewFileWriter(outputs, 2)
		if err != nil {
			panic(err.Error())
		}

		WriteStubs(w, aliases)
		_ = w.Close()
	}

	// Api
	{
		w, err := bindgen.NewFileWriter(outputs, 3)
		if err != nil {
			panic(err.Error())
		}

		WriteApi(w, aliases, activeExtensions)
		WriteApiLoad(w, versions, activeExtensions, extensionCommands)
		WriteApiInitStubs(w, aliases)

		_ = w.Close()
	}

	// Api load versions
	{
		w, err := bindgen.NewFileWriter(outputs, 4)
		if err != nil {
			panic(err.Error())
		}

		for _, version := range versions {
			WriteApiLoadVersion(w, version, versionCommands[version])
		}

		_ = w.Close()
	}

	// Api load extensions
	{
		w, err := bindgen.NewFileWriter(outputs, 5)
		if err != nil {
			panic(err.Error())
		}

		for _, ext := range activeExtensions {
			if len(extensionCommands[ext]) == 0 {
				continue
			}

			WriteApiLoadExtension(w, ext, extensionCommands[ext])
		}

		_ = w.Close()
	}

	// Api wrappers
	{
		w, err := bindgen.NewFileWriter(outputs, 6)
		if err != nil {
			panic(err.Error())
		}

		WriteApiMethodWrappers(w, aliases)
		_ = w.Close()
	}
}

func MapCustomType(enums map[string]*fb.Enum) func(str string) fb.Type {
	return func(str string) fb.Type {
		// Enum
		if enum, ok := enums[str]; ok {
			return &fb.DeclType{Decl: enum}
		}

		// Simple
		text := ""

		switch str {
		// --- Primitives ---
		case "GLboolean":
			text = "bool"
		case "GLbyte":
			text = "i8"
		case "GLubyte":
			text = "u8"
		case "GLshort":
			text = "i16"
		case "GLushort":
			text = "u16"
		case "GLint", "int":
			text = "i32"
		case "GLuint", "uint":
			text = "u32"
		case "GLfixed":
			text = "i32"
		case "GLint64", "GLint64EXT":
			text = "i64"
		case "GLuint64", "GLuint64EXT":
			text = "u64"
		case "GLsizei":
			text = "i32"
		case "GLenum":
			text = "u32"
		case "GLbitfield":
			text = "u32"
		case "GLfloat", "float":
			text = "f32"
		case "GLclampf":
			text = "f32"
		case "GLdouble", "double":
			text = "f64"
		case "GLclampd":
			text = "f64"

		// --- 16-bit Floats (Half) ---
		case "GLhalf", "GLhalfARB", "GLhalfNV":
			text = "u16" // 16-bit IEEE float representation

		// --- Strings & Raw Bytes ---
		case "GLchar", "GLcharARB", "char":
			text = "u8"

		// --- Void ---
		case "GLvoid", "void":
			text = "void"

		// --- Pointer-sized Integers (ptrdiff_t / size_t) ---
		case "GLsizeiptr", "GLsizeiptrARB":
			text = "u64"
		case "GLintptr", "GLintptrARB", "GLvdpauSurfaceNV":
			text = "i64"

		// --- Handles & Opaque Pointers (C: void* or uint) ---
		case "GLsync":
			text = "u64" // struct __GLsync*
		case "GLhandleARB":
			text = "u32" // 32-bit object handle in modern GL
		case "GLeglImageOES", "GLeglClientBufferEXT":
			text = "u64" // Opaque C: void*
		case "_cl_context", "_cl_event":
			text = "u64" // OpenCL interop pointers: struct _cl_*

		// --- Callbacks (Function Pointers) ---
		case "GLDEBUGPROC", "GLDEBUGPROCARB", "GLDEBUGPROCKHR", "GLDEBUGPROCAMD":
			text = "opengl::GlDebugProc" // Or your callback function pointer type
		case "GLVULKANPROCNV":
			return &fb.PointerType{Pointee: &fb.SimpleType{Text: "void"}}
		}

		if text != "" {
			return &fb.SimpleType{Text: text}
		}

		panic("failed to map type: " + str)
	}
}

func ParseUint(str string) uint64 {
	if strings.HasPrefix(str, "0x") {
		val, err := strconv.ParseUint(str[2:], 16, 64)
		if err != nil {
			panic(err.Error())
		}

		return val
	}

	val, err := strconv.ParseUint(str, 16, 64)
	if err != nil {
		panic(err.Error())
	}

	return val
}

func (reg *Registry) GetActiveItems(versions []Version) (map[string]Enum, map[string]Command, map[Version][]string, []string, map[string][]string) {
	activeEnums := make(map[string]Enum)
	activeCommands := make(map[string]Command)
	versionCommands := make(map[Version][]string)

	var activeExtensions []string
	extensionCommands := make(map[string][]string)

	applyRequire := func(version Version, ext string, require Require) {
		if require.Profile == "" || require.Profile == "core" {
			for _, enum := range require.Enums {
				activeEnums[enum.Name] = reg.GetEnum(enum.Name)
			}

			for _, command := range require.Commands {
				activeCommands[command.Name] = reg.GetCommand(command.Name)

				if (version != Version{}) {
					commands := versionCommands[version]
					commands = append(commands, command.Name)
					versionCommands[version] = commands
				} else if ext != "" {
					commands := extensionCommands[ext]
					commands = append(commands, command.Name)
					extensionCommands[ext] = commands
				}
			}
		}
	}

	applyRemove := func(remove Remove) {
		if remove.Profile == "" || remove.Profile == "core" {
			for _, enum := range remove.Enums {
				delete(activeEnums, enum.Name)
			}

			for _, command := range remove.Commands {
				delete(activeCommands, command.Name)

				for version := range versionCommands {
					commands := versionCommands[version]
					i := slices.Index(commands, command.Name)

					if i != -1 {
						commands = slices.Delete(commands, i, i+1)
						versionCommands[version] = commands
					}
				}

				for ext := range extensionCommands {
					commands := extensionCommands[ext]
					i := slices.Index(commands, command.Name)

					if i != -1 {
						commands = slices.Delete(commands, i, i+1)
						extensionCommands[ext] = commands
					}
				}
			}
		}
	}

	// Versions
	for _, version := range versions {
		feature := reg.GetFeature(version)

		for _, require := range feature.Requires {
			applyRequire(version, "", require)
		}

		for _, remove := range feature.Removes {
			applyRemove(remove)
		}
	}

	// Extensions
	for _, ext := range reg.Extensions {
		if (strings.HasPrefix(ext.Name, "GL_KHR_") || strings.HasPrefix(ext.Name, "GL_ARB_") || strings.HasPrefix(ext.Name, "GL_EXT_")) &&
			slices.Contains(ext.Supported, "glcore") {
			for _, require := range ext.Requires {
				applyRequire(Version{}, ext.Name, require)
			}

			for _, remove := range ext.Removes {
				applyRemove(remove)
			}

			activeExtensions = append(activeExtensions, ext.Name)
		}
	}

	slices.Sort(activeExtensions)

	return activeEnums, activeCommands, versionCommands, activeExtensions, extensionCommands
}

func (reg *Registry) GetEnum(name string) Enum {
	for _, group := range reg.Groups {
		for _, enum := range group.Enums {
			if enum.Name == name {
				return enum
			}
		}
	}

	panic("unknown enum for name: " + name)
}

func (reg *Registry) GetCommand(name string) Command {
	for _, command := range reg.Commands {
		if command.Proto.Name == name {
			return command
		}
	}

	panic("unknown command for name: " + name)
}

func (reg *Registry) GetFeature(version Version) Feature {
	for _, feature := range reg.Features {
		if feature.Api == "gl" && feature.Version == version {
			return feature
		}
	}

	panic("unknown feature for version: " + version.String())
}

func (reg *Registry) GetVersions() []Version {
	var versions []Version

	for _, feature := range reg.Features {
		if feature.Api == "gl" && !slices.Contains(versions, feature.Version) {
			versions = append(versions, feature.Version)
		}
	}

	slices.SortFunc(versions, func(a, b Version) int {
		return a.CompareTo(b)
	})

	return versions
}
