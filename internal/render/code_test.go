package render

import (
	"testing"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/internal/render/assets"
	"github.com/Olian04/webui/test/util/assert"
)

// A file of the editor is read, and compressed, once however many requests ask for it: it is the largest
// thing the app serves, and compressing it again for every request that accepts gzip cost over a hundred
// milliseconds each.
func TestAnEditorFileIsMadeOnceAndTheSameBlobIsServedAfter(t *testing.T) {
	t.Parallel()

	r, err := New(&ir.App{}, "", []byte("0123456789abcdef"))
	assert.NoError(t, err)
	files, err := assets.MonacoFiles()
	assert.NoError(t, err)

	first, ok := r.MonacoFile(files.Entry)
	assert.True(t, ok)
	second, _ := r.MonacoFile(files.Entry)
	assert.True(t, first == second)
	assert.NotNil(t, first.Gzip)
	zipped := first.Gzip()
	assert.True(t, &zipped[0] == &first.Gzip()[0]) // compressed once: the same bytes each time

	_, ok = r.MonacoFile("nothing.js")
	assert.False(t, ok)
}
