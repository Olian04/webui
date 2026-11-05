// Command precompress writes a Brotli copy beside each text file under the directories it is given: name.js gets name.js.br, at the best level, which is slow, and which is
// why it is done when the files are built and not when they are served. The renderer serves the copy to a client that takes Brotli. It is in the module of the tools, which is
// not the library's, so the library does not carry it.
//
//	go run ./precompress ../internal/render/assets/min ../pkg/monaco/editor
//
// A copy is written only where it is smaller, and a copy whose file is gone is removed, so that what is committed is what the files are.
package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/andybalholm/brotli"
)

// compressible is the files that gzip and Brotli make smaller: text, and TrueType, which is not compressed inside. WOFF and images are compressed already.
var compressible = map[string]bool{".js": true, ".css": true, ".json": true, ".svg": true, ".ttf": true}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: precompress <dir>...")
		os.Exit(2)
	}
	var files, bytesIn, bytesOut int
	for _, dir := range os.Args[1:] {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if source, ok := strings.CutSuffix(path, ".br"); ok {
				if _, err := os.Stat(source); err != nil {
					return os.Remove(path) // its file is gone
				}
				return nil
			}
			if !compressible[filepath.Ext(path)] {
				return nil
			}
			raw, err := os.ReadFile(path) //nolint:gosec // G304: the files of the directories it was given
			if err != nil {
				return err
			}
			var buf bytes.Buffer
			zw := brotli.NewWriterLevel(&buf, brotli.BestCompression)
			if _, err := zw.Write(raw); err != nil {
				return err
			}
			if err := zw.Close(); err != nil {
				return err
			}
			if buf.Len() >= len(raw) {
				if err := os.Remove(path + ".br"); err != nil && !os.IsNotExist(err) {
					return err
				}
				return nil // it does not shrink: it is not offered
			}
			files, bytesIn, bytesOut = files+1, bytesIn+len(raw), bytesOut+buf.Len()
			return os.WriteFile(path+".br", buf.Bytes(), 0o644) //nolint:gosec // G306: a file of the repository, like the one beside it
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	fmt.Printf("%d files, %d bytes in %d\n", files, bytesIn, bytesOut)
}
