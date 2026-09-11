# webui

Declarative admin panels and control planes. No HTML, CSS or JavaScript — only Go.

## WIP

```go
package main

import (
 "context"
 "errors"
 "log"
 "net/http"

 "github.com/Olian04/webui/pkg/webui"
)

var Devices = webui.Page[struct{}]{
 Path: webui.Path("device"),
 Nav: webui.Nav{
  Label: "Devices",
 },
 Sections: []webui.Section{
  webui.Table[Device]{
   Load: func(ctx context.Context) ([]Device, error) {
    return service.LoadAll()
   },
   RowClick: func(d Device) webui.Action {
    return webui.Redirect(Details, d.Id)
   },
   // No "Actions" or "BulkActions" means no action bar
   // No "BulkActions" means no row select checkboxes
   Columns: []webui.Column{
    webui.String("id", func(d Device) string { return d.Id }),
    webui.String("ip", func(d Device) string { return d.Ip }),
    webui.Int("occurances", func(d Device) int { return d.Count }),
    webui.Float("rate", func(d Device) float64 { return float64(d.Count) / d.Duration }),
   },
  },
 },
}

var Details = webui.Page[string]{
 // :args: instructs that the args (a single string this time) should be used as part of the path.
 // :args.Id: or :args.Count: if Args is a struct with Id and Count properties
 // Any args not in the path will be treated as query parameters.
 // Args with value equal to its zero value will be passed literally when used in path, and removes the query param otherwise.
 Path: webui.Path("device", ":args:"),
 Nav: webui.Nav{
  Shadow: Devices.Nav, // Doesn't show up in the Navbar, but shows as being on the "Device" entry when page is loaded
 },
 Sections: []webui.Section{
  webui.Form[Device]{
   Load: func(ctx context.Context) (Device, error) {
    id, err := Details.Args(ctx)
    if err != nil || id == "" {
     return Device{}, errors.New("no device id provided")
    }
    return service.Load(id)
   },
   Store: func(ctx context.Context, d Device) error {
    return service.Store(d.Id, d)
   },
   Fields: []webui.Field{
    webui.Group{
     webui.StringField{
      Label: "ID",
      Load:  func(d Device) string { return d.Id },
      // No "Store" == Read-only
     },
     webui.StringField{
      Label: "IP",
      Load: func(d Device) string { return d.Ip },
      Store: func(d Device, val string) error {
       d.Ip = val
       return nil
      },
     },
    },
    webui.FloatField{
     Label: "Rate",
     Load:  func(d Device) float64 { return float64(d.Count) / d.Duration },
    },
   },
  },
 },
}

func main() {
 app := webui.App{
  Brand: webui.Brand{
   Name: "Demo",
   Logo: image.Load("./resources/logo.svg"),
  },
  Theme: webui.ThemeDark,
  Pages: []webui.Page{
   Devices,
   Details,
  },
 }

 if err := app.Validate(); err != nil {
  log.Fatal(err)
 }

 mux := http.NewServeMux()

 if err := app.Mount(mux, "/admin", authMiddleware); err != nil {
  log.Fatal(err)
 }

 // Your own routes coexist; the library owns exactly its own subtree.
 mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
  w.WriteHeader(http.StatusOK)
 })

 log.Fatal(http.ListenAndServe(":8080", mux))
}
```
