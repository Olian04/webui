package webui_test

import (
	"context"
	"fmt"

	"github.com/Olian04/webui/pkg/webui"
)

// Compile reports every problem in the declaration at once, each with where it is
// and how to fix it.
func ExampleApp_Compile() {
	type deviceArgs struct{ Name string } // the path below names {id}
	type device struct{ ID string }

	broken := webui.Page[deviceArgs]{
		Path: "/device/{id}",
		Body: webui.Form[device]{
			Load: func(context.Context) (device, error) { return device{}, nil },
		},
	}

	_, err := webui.App{Pages: webui.Pages{broken}}.Compile("/admin")
	fmt.Println(err)
	// Output:
	// page "/device/{id}" (deviceArgs): the path declares {id} but deviceArgs has no field for it
	//   Fix: Add a field named Id to deviceArgs, or change the placeholder to match an existing field.
}
