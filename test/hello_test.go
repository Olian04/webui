package webui

import (
	"image"
	"net/http"
	"net/url"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
	"github.com/Olian04/webui/test/util/mock"
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

	mux := mock.NewMux()
	mux.Handle(helloWorldPath, handler)

	response := mux.Serve(&http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Path: helloWorldPath},
	})

	assert.Equal(t, response.StatusCode, http.StatusOK)
	assert.Contains(t, response.Body.String(), app.Brand.Name)
}
