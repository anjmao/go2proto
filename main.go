package main

import (
	"flag"
	"go/token"
	"go/types"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"
)

type arrFlags []string

func (i *arrFlags) String() string {
	return ""
}

func (i *arrFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var (
	filter      = flag.String("filter", "", "Filter struct names.")
	protoFolder = flag.String("f", "", "Proto output path.")
	pkgFlags    arrFlags
)

func main() {
	flag.Var(&pkgFlags, "p", "Go source packages.")
	flag.Parse()

	if len(pkgFlags) == 0 || protoFolder == nil {
		flag.PrintDefaults()
		os.Exit(1)
	}

	if err := checkOutFolder(*protoFolder); err != nil {
		log.Fatal(err)
	}

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	pkgs, err := loadPackages(pwd, pkgFlags)
	if err != nil {
		log.Fatal(err)
	}

	msgs := getMessages(pkgs, *filter)

	if err := writeOutput(msgs, *protoFolder); err != nil {
		log.Fatal(err)
	}
}

func checkOutFolder(path string) error {
	_, err := os.Stat(path)
	return err
}

func loadPackages(pwd string, pkgs []string) ([]*packages.Package, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Dir:  pwd,
		Mode: packages.LoadSyntax,
		Fset: fset,
	}
	return packages.Load(cfg, pkgs...)
}

type message struct {
	Name   string
	Fields []*field
}

type field struct {
	Name       string
	TypeName   string
	Order      int
	IsRepeated bool
}

func getMessages(pkgs []*packages.Package, filter string) []*message {
	var out []*message
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
				if filter == "" || strings.Contains(t.Name(), filter) {
					out = appendMessage(out, t, s)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func appendMessage(out []*message, t types.Object, s *types.Struct) []*message {
	msg := &message{
		Name:   t.Name(),
		Fields: []*field{},
	}

	for i := 0; i < s.NumFields(); i++ {
		f := s.Field(i)
		if !f.Exported() {
			continue
		}
		newField := &field{
			Name:       toProtoFieldName(f.Name()),
			TypeName:   toProtoFieldTypeName(f),
			IsRepeated: isRepeated(f),
			Order:      i + 1,
		}
		msg.Fields = append(msg.Fields, newField)
	}
	out = append(out, msg)
	return out
}

func toProtoFieldTypeName(f *types.Var) string {
	switch f.Type().Underlying().(type) {
	case *types.Basic:
		name := f.Type().String()
		return normalizeType(name)
	case *types.Slice:
		name := splitNameHelper(f)
		return normalizeType(strings.TrimLeft(name, "[]"))

	case *types.Pointer, *types.Struct:
		name := splitNameHelper(f)
		return normalizeType(name)
	}
	return f.Type().String()
}

func splitNameHelper(f *types.Var) string {
	// TODO: this is ugly. Find another way of getting field type name.
	parts := strings.Split(f.Type().String(), ".")

	name := parts[len(parts)-1]

	if name[0] == '*' {
		name = name[1:]
	}
	return name
}

func normalizeType(name string) string {
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

func isRepeated(f *types.Var) bool {
	_, ok := f.Type().Underlying().(*types.Slice)
	return ok
}

func toProtoFieldName(name string) string {
	if len(name) == 2 {
		return strings.ToLower(name)
	}
	r, n := utf8.DecodeRuneInString(name)
	return string(unicode.ToLower(r)) + name[n:]
}

func writeOutput(msgs []*message, path string) error {
	msgTemplate := `syntax = "proto3";
package proto;

{{range .}}
message {{.Name}} {
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

	f, err := os.Create(filepath.Join(path, "output.proto"))
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, msgs)
}
