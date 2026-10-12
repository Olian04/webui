package main

import (
	"embed"
	"io/fs"
)

// The language services of the editors are given the files in context/: TypeScript the
// types the collector hands a rule and the options to check them under, JSON the schema of
// the configuration. It is one folder with a directory for each language, and an editor
// reads only the one for its own Language. Embedding it is what makes it part of the program.
//
//go:embed context
var contextFiles embed.FS

var editorContext = func() fs.FS {
	sub, err := fs.Sub(contextFiles, "context")
	if err != nil {
		panic(err)
	}
	return sub
}()
