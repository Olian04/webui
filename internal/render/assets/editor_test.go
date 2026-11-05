package assets

import (
	"testing"

	"github.com/Olian04/webui/test/util/assert"
)

// The editor is not in the files that every program carries: it is registered by pkg/monaco when a program imports it, and until then there is none to read or to serve.
func TestTheEditorIsNotInTheFilesEveryProgramCarries(t *testing.T) {
	t.Parallel()

	assert.False(t, EditorInstalled())
	_, err := MonacoFiles()
	assert.Error(t, err)
	_, ok := MonacoFile("code.js")
	assert.False(t, ok)
	_, err = FS.ReadFile("monaco/manifest.json")
	assert.Error(t, err)
}
