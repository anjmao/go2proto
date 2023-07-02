package main

import (
	"flag"
	"github.com/sergeyglazyrindev/go2proto/importable"
	"log"
	"os"
	"strings"
)

const outputFileName = "output.proto"

var (
	filter       = flag.String("filter", "", "Filter by struct names. Case insensitive.")
	targetFolder = flag.String("f", ".", "Protobuf output file path.")
	pkgFlags     importable.ArrFlags
)

func main() {
	flag.Var(&pkgFlags, "p", `Fully qualified path of packages to analyse. Relative paths ("./example/in") are allowed.`)
	flag.Parse()

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("error getting working directory: %s", err)
	}

	if len(pkgFlags) == 0 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	//ensure the path exists
	_, err = os.Stat(*targetFolder)
	if err != nil {
		log.Fatalf("error getting output file: %s", err)
	}

	pkgs, err := importable.LoadPackages(pwd, pkgFlags)
	if err != nil {
		log.Fatalf("error fetching packages: %s", err)
	}

	msgs := importable.GetMessages(pkgs, strings.ToLower(*filter))

	if err = importable.WriteToFile(msgs, *targetFolder, outputFileName); err != nil {
		log.Fatalf("error writing output: %s", err)
	}

	log.Printf("output file written to %s%s%s\n", pwd, string(os.PathSeparator), outputFileName)
}
