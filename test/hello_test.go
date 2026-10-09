package webui_test

import (
	"image"
	"net/http"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

const (
	helloWorldPath = "/admin"
)

func TestHelloWorld(t *testing.T) {
	t.Parallel()

	app := webui.App{
		Brand: webui.Brand{
			Name: "Hello World",
			Logo: image.Image(nil),
		},
	}

	handler, err := app.Compile(helloWorldPath)
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	response := serve(handler, http.MethodGet, helloWorldPath)

	assert.Equal(t, response.Code, http.StatusOK)
	assert.Contains(t, response.Body.String(), app.Brand.Name)
}
