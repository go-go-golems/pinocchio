package main

import (
	"github.com/go-go-golems/pinocchio/pkg/analysis/sessionstreamschema"
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() {
	unitchecker.Main(sessionstreamschema.Analyzer)
}
