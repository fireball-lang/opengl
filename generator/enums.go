package main

import (
	"cmp"
	"slices"
	"strings"

	"github.com/fireball-lang/bindgen"
	"github.com/fireball-lang/bindgen/fb"
)

func GetFbEnums(reg Registry, active map[string]Enum) map[string]*fb.Enum {
	enums := make(map[string]*fb.Enum)

	// Populate enums and cases
	for _, active := range active {
		if active.Alias != "" {
			continue
		}

		for _, group := range active.Groups {
			enum, ok := enums[group]

			if !ok {
				enum = &fb.Enum{
					Name: group,
					Type: &fb.SimpleType{Text: "u32"},
				}

				enums[group] = enum
			}

			enum.Cases = append(enum.Cases, &fb.Case{
				Name:  active.Name,
				Value: active.Value,
			})

			if active.Type == "ull" {
				enum.Type.(*fb.SimpleType).Text = "u64"
			}
		}
	}

	// Set bitfield flag
	for _, group := range reg.Groups {
		if group.Kind == "bitmask" {
			if enum, ok := enums[group.Name]; ok {
				enum.Bitfield = true
			}
		}
	}

	// Deduplicate, rename and sort cases
	for _, enum := range enums {
		var newCases []*fb.Case

		for _, cas := range enum.Cases {
			ok := true

			if strings.HasSuffix(cas.Name, "_KHR") || strings.HasSuffix(cas.Name, "_ARB") || strings.HasSuffix(cas.Name, "_EXT") || strings.HasSuffix(cas.Name, "_RGB") {
				withoutSuffix := cas.Name[:len(cas.Name)-4]

				if slices.ContainsFunc(enum.Cases, func(c *fb.Case) bool {
					return c.Name == withoutSuffix && c.Value == cas.Value
				}) {
					ok = false
				}
			}

			if enum.Name == "SpecialNumbers" {
				if cas.Name != "GL_INVALID_INDEX" && cas.Name != "GL_TIMEOUT_IGNORED" {
					ok = false
				}
			}

			if ok {
				newCases = append(newCases, cas)
			}
		}

		for _, cas := range newCases {
			name := strings.TrimPrefix(cas.Name, "GL_")
			name = bindgen.SnakeToPascalCase(name)

			cas.Name = name
		}

		slices.SortFunc(newCases, func(a, b *fb.Case) int {
			aValue := ParseUint(a.Value)
			bValue := ParseUint(b.Value)

			return cmp.Compare(aValue, bValue)
		})

		enum.Cases = newCases
	}

	return enums
}

func WriteExtensionEnum(w fb.Writer, extensions []string) {
	w.Write("\n")
	w.Write("pub enum Extension {\n")

	for _, ext := range extensions {
		name := strings.TrimPrefix(ext, "GL_")
		name = bindgen.SnakeToPascalCase(name)

		w.Write("    %s,\n", name)
	}

	w.Write("}\n")
}

func WriteEnums(w fb.Writer, enums map[string]*fb.Enum) {
	// Sort
	sortedEnums := make([]*fb.Enum, 0, len(enums))

	for _, enum := range enums {
		sortedEnums = append(sortedEnums, enum)
	}

	slices.SortFunc(sortedEnums, func(a, b *fb.Enum) int {
		return cmp.Compare(a.Name, b.Name)
	})

	// Write
	for _, decl := range sortedEnums {
		w.Write("\n")
		decl.Write(w)
	}
}
