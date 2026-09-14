package main

import (
	"cmp"
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Registry struct {
	Groups     []Group     `xml:"enums"`
	Commands   []Command   `xml:"commands>command"`
	Features   []Feature   `xml:"feature"`
	Extensions []Extension `xml:"extensions>extension"`
}

type Group struct {
	Name string `xml:"group,attr"`
	Kind string `xml:"type,attr"`

	Enums []Enum `xml:"enum"`
}

type Enum struct {
	Name  string
	Alias string

	Type  string
	Value string

	Groups []string
}

func (e *Enum) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "name":
			e.Name = attr.Value
		case "alias":
			e.Alias = attr.Value
		case "type":
			e.Type = attr.Value
		case "value":
			e.Value = attr.Value
		case "group":
			e.Groups = strings.Split(attr.Value, ",")
		}
	}

	return d.Skip()
}

type Command struct {
	Proto  Prototype `xml:"proto"`
	Params []Param   `xml:"param"`
}

type Prototype struct {
	Name string

	Group   string
	Returns string
}

func (p *Prototype) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "group":
			p.Group = attr.Value
		}
	}

	return unmarshalNameType(d, start, &p.Name, &p.Returns)
}

type Param struct {
	Group string
	Kind  string

	Name string
	Type string
}

func (p *Param) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "group":
			p.Group = attr.Value
		case "kind":
			p.Kind = attr.Value
		}
	}

	return unmarshalNameType(d, start, &p.Name, &p.Type)
}

type Feature struct {
	Api     string  `xml:"api,attr"`
	Version Version `xml:"number,attr"`

	Requires []Require `xml:"require"`
	Removes  []Remove  `xml:"remove"`
}

type Extension struct {
	Name      string        `xml:"name,attr"`
	Supported SupportedAPIs `xml:"supported,attr"`

	Requires []Require `xml:"require"`
	Removes  []Remove  `xml:"remove"`
}

type SupportedAPIs []string

func (s *SupportedAPIs) UnmarshalText(text []byte) error {
	*s = strings.Split(string(text), "|")
	return nil
}

type Require struct {
	Profile string `xml:"profile,attr"`

	Enums []struct {
		Name string `xml:"name,attr"`
	} `xml:"enum"`
	Commands []struct {
		Name string `xml:"name,attr"`
	} `xml:"command"`
}

type Remove struct {
	Profile string `xml:"profile,attr"`

	Enums []struct {
		Name string `xml:"name,attr"`
	} `xml:"enum"`
	Commands []struct {
		Name string `xml:"name,attr"`
	} `xml:"command"`
}

type Version struct {
	Major int
	Minor int
}

func (v *Version) UnmarshalText(text []byte) error {
	parts := strings.Split(string(text), ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid version format %q (expected X.Y)", string(text))
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid major version: %w", err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid minor version: %w", err)
	}

	v.Major = major
	v.Minor = minor

	return nil
}

func (v *Version) CompareTo(o Version) int {
	major := cmp.Compare(v.Major, o.Major)
	if major != 0 {
		return major
	}

	return cmp.Compare(v.Minor, o.Minor)
}

func (v *Version) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

func LoadRegistry() Registry {
	file, err := os.Open("gl.xml")
	if err != nil {
		panic(err.Error())
	}

	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	var registry Registry

	if err := xml.NewDecoder(file).Decode(&registry); err != nil {
		panic(err.Error())
	}

	return registry
}

func unmarshalNameType(d *xml.Decoder, start xml.StartElement, name, typ *string) error {
	var fullType strings.Builder

	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "name":
				var str string
				if err := d.DecodeElement(&str, &t); err != nil {
					return err
				}

				*name = strings.TrimSpace(str)

			case "ptype":
				var str string
				if err := d.DecodeElement(&str, &t); err != nil {
					return err
				}

				fullType.WriteString(str)

			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}

		case xml.CharData:
			fullType.Write(t)

		case xml.EndElement:
			if t == start.End() {
				*typ = strings.Join(strings.Fields(fullType.String()), " ")
				return nil
			}
		}
	}
}
