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

var DevicesNavItem = webui.Nav{
  Label: "Devices",
}

var Devices = webui.Page[any]{
 Path: webui.Path().Static("device"),
 Nav: DevicesNavItem,
 Sections: []webui.Section{
  webui.Table[Device]{
   Load: func(ctx context.Context) ([]Device, error) {
    return service.LoadAll()
   },
   RowClick: EditDevice,
   // No "Actions" or "BulkActions" means no action bar
   // No "BulkActions" means no row select checkboxes
   Columns: []webui.Column{
    webui.String("id", func(d Device) string { return d.Id }),
    webui.String("ip", func(d Device) string { return d.Ip }),
    webui.Int("occurrences", func(d Device) int { return d.Count }),
    webui.Float("rate", func(d Device) float64 { return float64(d.Count) / d.Duration }),
   },
  },
 },
}

var EditDevice = webui.Action[Device]{
  Run: func(d Device) webui.Effect {
    return Details.Open(d.Id)
  },
}

var Details = webui.Page[string]{
 Path: webui.Path().Static("device").Arg(func(id string) string { return id }), // /device/{arg}
 Nav: webui.Nav{
  Shadow: DevicesNavItem, // Doesn't show up in the Navbar, but shows as being on the "Device" entry when page is loaded
 },
 Sections: []webui.Section{
  webui.Form[Device]{
   Load: func(ctx context.Context) (Device, error) {
    id, err := Details.GetArg(ctx)
    if err != nil || id == "" {
     return Device{}, errors.New("no device id provided")
    }
    return service.Load(id)
   },
   Submit: SaveDevice,
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
      Store: func(d *Device, val string) error {
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

var SaveDevice = webui.Action[Device]{
  // Guard is run on page load to enable UI elements based on access, it is also ran on action execution before Run to guard requests.
  Guard: func(ctx context.Context) error {
    return auth.AssertRole(ctx, auth.EditorRole)
  },
  Run: func(d Device) webui.Effect {
    if err := service.Put(d.Id, d); err != nil {
      return webui.Effect{
        Error: err, // Submit failed, restore form fields and show error message
      }
    }
    return webui.Effect{
      Message: "Device saved!", // Submit was successful, show message as toast.
    }
  },
}

func main() {
 app := webui.App{
  Brand: webui.Brand{
   Name: "Demo",
   Logo: image.Load("./resources/logo.svg"),
  },
  Theme: webui.Theme{}, // Default theme
  Pages: webui.Pages{
   Devices,
   Details,
  },
 }

 if err := app.Validate(); err != nil {
  log.Fatal(err)
 }

 mux := http.NewServeMux()
 mux.Handle("/admin/", authMiddleware(http.StripPrefix("/admin", app.HttpHandler("/admin"))))

 // Your own routes coexist; the library owns exactly its own subtree.
 mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
  w.WriteHeader(http.StatusOK)
 })

 log.Fatal(http.ListenAndServe(":8080", mux))
}
```
