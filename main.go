package main

import (
	"flag"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/packages"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"
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

	msgs := getMessages(pkgs)

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

func getMessages(pkgs []*packages.Package) []*message {
	out := []*message{}
	for _, p := range pkgs {
		for _, t := range p.TypesInfo.Defs {
			if t == nil {
				continue
			}
			if !t.Exported() {
				continue
			}
			if s, ok := t.Type().Underlying().(*types.Struct); ok {
				out = appendMessage(out, t, s)
			}
		}

	}
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
		return f.Type().String()
	case *types.Slice, *types.Pointer:
		parts := strings.Split(f.Type().String(), ".")
		return parts[len(parts)-1]
	}
	return f.Type().String()
}

func isRepeated(f *types.Var) bool {
	_, ok := f.Type().Underlying().(*types.Slice)
	return ok
}

func toProtoFieldName(name string) string {
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

	return tmpl.Execute(f, msgs)
}
