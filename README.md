# webui

Declarative admin panels and control planes. No HTML, CSS or JavaScript — only Go.

## WIP

```go
package main

import (
	"log"
	"net/http"
	
	"github.com/Olian04/webui/pkg/webui"
)

Devices := webui.Page{
   Path: webui.Path("device")
   Nav: webui.Nav{
      Label: "Devices",
   },
   Sections: []webui.Section{
      webui.Table[Device]{
          Load: func (ctx context.Context) ([]Device, error) {
              return service.LoadAll()
          },
          RowClick: func (d Device) webui.Action {
            return webui.Redirect(Details, DeviceId.Is(d.Id))
          },
          // No "Actions" or "BulkActions" means no action bar
          // No "BulkActions" means no row select checkboxes
          Columns: []webui.Column{
              webui.String("id", func (d Device) string { return d.Id })
              webui.String("ip", func (d Device) string { return d.Ip })
              webui.Int("occurances", func (d Device) int { return d.Count })
              webui.Float("rate", func (d Device) float { return d.Count / d.Duration })
          }
      }
   }
}

DeviceId := webui.Var[String]() 
Details := webui.Page{
  Path: webui.Path("device", DeviceId),
  // Query: webui.Query{ "id": DeviceId },
  Nav: webui.Nav{
    Shadow: Devices.Nav, // Doesn't show up in the Navbar, but shows as being on the "Device" entry when page is loaded
  },
  Sections: []webui.Section{
    webui.Form[Device]{
      Load: func (ctx context.Context) (Device, error) {
         deviceId, err := DeviceId.Get(ctx)
         if (err != nil) {
           return Device{}, error.New("No device id provided")
         }
         return service.Load(deviceId)
      },
      Store: func (ctx context.Context, d Device) error {
        return service.Store(d.Id, d)
      },
      Fields: []webui.Field{
        webui.Group{
          webui.StringField{
            Label: "ID",
            Load: func (d Device) string { return d.Id },
            // No "Store" == Read-only
          },
          webui.StringField{
            Label: "IP",
            Load: func (d Device) string { return d.Ip },
            Store: func (d Device, val string) error {
              d.Ip = val
              return nil
            }
          },
        },
        webui.FloatField{
          Label: "Rate",
          Load: func (d Device) float { return d.Count / d.Duration },
        }
      }
    }
  }
}

app := webui.App{
  Brand: webui.Brand{
    Name: "Demo", 
    Logo: image.Load("./resources/logo.svg")
  },
  Theme: webui.ThemeDark,
  Pages: []webui.Page{
     Devices,
     Details,
  }
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
```
