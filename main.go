package main

import (
	"errors"
	"flag"
	"fmt"
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

type commaStrings []string

const outputFileName = "output.proto"

func (i *commaStrings) String() string {
	return ""
}

func (i *commaStrings) Set(value string) error {
	*i = strings.Split(value, ",")
	return nil
}

var (
	filter       = flag.String("filter", "", "Filter by struct names. Case insensitive.")
	targetFolder = flag.String("f", ".", "Protobuf output directory path.")
	useTags      = flag.Bool("t", false, "Support tagging (requires tagger/tagger.proto plugin)")
	pkgPaths     commaStrings
)

func main() {
	flag.Var(&pkgPaths, "p", `Comma-separated paths of packages to analyse. Relative paths ("./example/in") are allowed.`)
	flag.Parse()

	if len(pkgPaths) == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	//ensure the output path exists and is a directory
	info, err := os.Stat(*targetFolder)

	if os.IsNotExist(err) {
		log.Fatalf("output folder %s does not exist", *targetFolder)
	}

	if info.Mode().IsRegular() {
		log.Fatalf("%s is not a directory", *targetFolder)
	}

	absTargetPath, err := filepath.Abs(*targetFolder)

	if err != nil {
		log.Fatalf("error getting absolute output folder: %s", err)
	}

	pkgs, err := loadPackages(pkgPaths)
	if err != nil {
		log.Fatalf("error fetching packages: %s", err)
	}

	msgs := getMessages(pkgs, strings.ToLower(*filter))

	if err = writeOutput(msgs, *targetFolder, *useTags); err != nil {
		log.Fatalf("error writing output: %s", err)
	}

	log.Printf("output file written to %s\n", filepath.Join(absTargetPath, outputFileName))
}

// attempt to load all packages
func loadPackages(pkgs []string) ([]*packages.Package, error) {

	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting working directory: %s", err)
	}

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

type protoData struct {
	UseTags  bool
	Messages []*message
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
	Tags       string
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
				if filter == "" || strings.Contains(strings.ToLower(t.Name()), filter) {
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
			Tags:       s.Tag(i),
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

func writeOutput(msgs []*message, path string, useTags bool) error {

	protobufTemplate := `{{- define "field" }}{{.TypeName}} {{.Name}} = {{.Order}}{{if writeTags . }} [(tagger.tags) = "{{escapeQuotes .Tags}}"]{{ end }};{{ end -}}
syntax = "proto3";

package proto;
{{- if importTagger}}

import "tagger/tagger.proto";{{end}}
{{range .}}
message {{.Name}} {
{{- range .Fields}}
  {{ if .IsRepeated}}repeated {{ end }}{{ template "field" . }}
{{- end}}
}
{{end}}
`
	f, err := os.Create(filepath.Join(path, outputFileName))
	if err != nil {
		return fmt.Errorf("unable to create file %s : %s", outputFileName, err)
	}
	defer f.Close()

	customFuncMap := template.FuncMap{
		"escapeQuotes": func(tag string) string {
			return strings.Replace(tag, `"`, `\"`, -1)
		},
		"writeTags": func(f field) bool {
			return useTags && f.Tags != ""
		},
		"importTagger": func() bool {
			if !useTags {
				return false
			}
			for _, msg := range msgs {
				for _, field := range msg.Fields {
					if field.Tags != "" {
						return true
					}
				}
			}
			return false
		},
	}

	tmpl, err := template.New("protobuf").Funcs(customFuncMap).Parse(protobufTemplate)
	if err != nil {
		panic(err)
	}

	return tmpl.Execute(f, msgs)
}
