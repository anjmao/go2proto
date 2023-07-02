package importable

import (
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"
)

type ArrFlags []string

func (i *ArrFlags) String() string {
	return ""
}

func (i *ArrFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

// attempt to load all packages

func LoadPackages(pwd string, pkgs []string) ([]*packages.Package, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Dir:  pwd,
		Mode: packages.LoadSyntax,
		Fset: fset,
	}
	packages, err := packages.Load(cfg, pkgs...)
	if err != nil {
		return nil, err
	}
	var errs = ""
	//check each loaded package for errors during loading
	for _, p := range packages {
		if len(p.Errors) > 0 {
			errs += fmt.Sprintf("error fetching package %s: ", p.String())
			for _, e := range p.Errors {
				errs += e.Error()
			}
			errs += "; "
		}
	}
	if errs != "" {
		return nil, errors.New(errs)
	}
	return packages, nil
}

type ProtoMessage struct {
	Name   string
	Fields []*MessageField
}

type MessageField struct {
	Name       string
	TypeName   string
	Order      int
	IsRepeated bool
}

func GetMessages(pkgs []*packages.Package, filter string) []*ProtoMessage {
	var out []*ProtoMessage
	seen := map[string]struct{}{}
	for _, p := range pkgs {
		for _, t := range p.TypesInfo.Defs {
			if t == nil {
				continue
			}
			if !t.Exported() {
				continue
			}
			if _, ok := seen[t.Name()]; ok {
				continue
			}
			if s, ok := t.Type().Underlying().(*types.Struct); ok {
				seen[t.Name()] = struct{}{}
				if filter == "" || strings.Contains(strings.ToLower(t.Name()), filter) {
					out = StructToProto(out, t, s)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func StructToProto(out []*ProtoMessage, t types.Object, s *types.Struct) []*ProtoMessage {
	msg := &ProtoMessage{
		Name:   t.Name(),
		Fields: []*MessageField{},
	}

	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if !f.Exported() {
			continue
		}
		newField := &MessageField{
			Name:       AdaptNameToProto(f.Name()),
			TypeName:   AdaptGoTypeToProto(f),
			IsRepeated: IsRepeated(f),
			Order:      i + 1,
		}
		msg.Fields = append(msg.Fields, newField)
	}
	out = append(out, msg)
	return out
}

func AdaptGoTypeToProto(f *types.Var) string {
	switch f.Type().Underlying().(type) {
	case *types.Basic:
		name := f.Type().String()
		return NormalizeType(name)
	case *types.Slice:
		name := SplitNameHelper(f)
		return NormalizeType(strings.TrimLeft(name, "[]"))

	case *types.Pointer, *types.Struct:
		name := SplitNameHelper(f)
		return NormalizeType(name)
	}
	return f.Type().String()
}

func SplitNameHelper(f *types.Var) string {
	// TODO: this is ugly. Find another way of getting MessageField type name.
	parts := strings.Split(f.Type().String(), ".")

	name := parts[len(parts)-1]

	if name[0] == '*' {
		name = name[1:]
	}
	return name
}

func NormalizeType(name string) string {
	switch name {
	case "int":
		return "int64"
	case "float32":
		return "float"
	case "float64":
		return "double"
	default:
		return name
	}
}

func IsRepeated(f *types.Var) bool {
	_, ok := f.Type().Underlying().(*types.Slice)
	return ok
}

func AdaptNameToProto(name string) string {
	if len(name) == 2 {
		return strings.ToLower(name)
	}
	r, n := utf8.DecodeRuneInString(name)
	return string(unicode.ToLower(r)) + name[n:]
}

func WriteToFile(msgs []*ProtoMessage, path string, outputFileName string) error {
	msgTemplate := `syntax = "proto3";
package proto;

{{range .}}
ProtoMessage {{.Name}} {
{{- range .Fields}}
{{- if .IsRepeated}}
  repeated {{.TypeName}} {{.Name}} = {{.Order}};
{{- else}}
  {{.TypeName}} {{.Name}} = {{.Order}};
{{- end}}
{{- end}}
}
{{end}}
`
	tmpl, err := template.New("test").Parse(msgTemplate)
	if err != nil {
		panic(err)
	}

	f, err := os.Create(filepath.Join(path, outputFileName))
	if err != nil {
		return fmt.Errorf("unable to create file %s : %s", outputFileName, err)
	}
	defer f.Close()

	return tmpl.Execute(f, msgs)
}
