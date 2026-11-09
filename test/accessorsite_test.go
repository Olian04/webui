package webui_test

import (
	"context"
	"testing"

	"github.com/Olian04/webui/pkg/webui"
	"github.com/Olian04/webui/test/util/assert"
)

// What an accessor may be depends on where it is, and each accessor says so of itself, by the site that it is checked in: a group is for the places that compose form fields, one is not
// nested, and it is checked as its accessors would be beside it.
func TestAGroupIsRefusedWhereItIsNotAllowedAndWhereItIsNested(t *testing.T) {
	t.Parallel()

	load := func(context.Context) (Device, webui.Loaded) { return Device{}, webui.Ready() }
	form := func(fields ...webui.Accessor[Device]) webui.View {
		return webui.Form[Device]{Load: load, Fields: fields}
	}

	refuses(t, editorErrors(t, form(webui.Group[Device]{webui.Group[Device]{formID}})), "Form.Fields[0][0]", "Group")
	refuses(t, editorErrors(t, webui.Table[Device]{
		Loader:  webui.RowsLoader[Device]{Load: func(context.Context) ([]Device, webui.Loaded) { return nil, webui.Ready() }},
		Columns: []webui.Accessor[Device]{webui.Group[Device]{formID}},
	}), "Table.Columns[0]", "Group")

	// The accessors in a group are checked as they are beside it: a label that is missing is as missing in a group as out of one.
	refuses(t, editorErrors(t, form(webui.Group[Device]{webui.String[Device]{Load: formID.Load}})), "Form.Fields[0][0]", "Label")
}

// A URL is a link of its own, so it is the first column of a table only where the rows are not links or buttons: it is the site that says so, and not the table's look at its first column.
func TestAURLIsRefusedAsTheFirstColumnOfATableWhoseRowsAreLinksAndNowhereElse(t *testing.T) {
	t.Parallel()

	rows := webui.RowsLoader[Device]{Load: func(context.Context) ([]Device, webui.Loaded) { return nil, webui.Ready() }}
	link := webui.URL[Device]{Label: "Site", Load: func(d Device) string { return d.Ip }}
	click := webui.Href[Device]{URL: func(d Device) string { return "/" + d.Id }}

	refuses(t, editorErrors(t, webui.Table[Device]{Loader: rows, RowClick: click, Columns: []webui.Accessor[Device]{link, formID}}), "URL", "RowClick")
	compiles := func(body webui.View) error {
		_, err := webui.App{Pages: webui.Pages{webui.Page[webui.NoArgs]{Path: "/e", View: body}}}.Compile("/admin")
		return err
	}
	assert.NoError(t, compiles(webui.Table[Device]{Loader: rows, RowClick: click, Columns: []webui.Accessor[Device]{formID, link}})) // the second column is no one's cover
	assert.NoError(t, compiles(webui.Table[Device]{Loader: rows, Columns: []webui.Accessor[Device]{link, formID}}))                  // and with no click, the first is free too
}
