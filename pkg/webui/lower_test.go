package webui

import (
	"context"
	"testing"

	"github.com/Olian04/webui/internal/ir"
	"github.com/Olian04/webui/test/util/assert"
)

type dev struct {
	Id, Ip string
	Count  int
}

type detailArgs struct {
	Id string
}

type listArgs struct {
	Q string
}

func fixture() App {
	id := String[dev]{Label: "ID", Load: func(d dev) string { return d.Id }}
	ip := String[dev]{
		Label: "IP", Load: func(d dev) string { return d.Ip }, Store: func(d *dev, v string) { d.Ip = v },
		Rules: StringRules{Required: true, MinLen: 7, Pattern: &PatternRule{Expr: `^\d+(\.\d+){3}$`, Message: "must be IPv4"}},
	}
	count := Int[dev]{Label: "Count", Load: func(d dev) int { return d.Count }, Store: func(d *dev, v int) { d.Count = v }}
	details := Page[detailArgs]{
		Path: "/device/{id}",
		Body: Tabs{Panels: []Tab{
			{Label: "Overview", Body: Form[dev]{
				Load:   func(context.Context) (dev, error) { return dev{Id: "d1", Ip: "1.2.3.4"}, nil },
				Fields: []Accessor[dev]{Group[dev]{id, ip}, count},
				Submit: Action[dev]{Run: func(context.Context, dev) (Outcome, error) { return Outcome{}, nil }},
			}},
			{Label: "Raw", Body: Stack{Split{Table[dev]{Load: func(context.Context, Query) (Rows[dev], error) { return Rows[dev]{}, nil }}}}},
		}},
	}
	list := Page[listArgs]{
		Path: "/device",
		Nav:  Nav{Label: "Devices"},
		Body: Table[dev]{
			Load: func(context.Context, Query) (Rows[dev], error) {
				return Rows[dev]{Items: []dev{{Id: "a"}, {Id: "b"}}}, nil
			},
			ID:       "devices",
			PageSize: 2,
			Key:      func(d dev) string { return d.Id },
			RowClick: Link[dev, detailArgs]{
				Page: details,
				Args: func(_ context.Context, d dev) detailArgs { return detailArgs{Id: d.Id} },
			},
			Columns: []Accessor[dev]{String[dev]{Label: "ID", Load: func(d dev) string { return d.Id }}, ip},
			BulkActions: []Action[[]dev]{{Label: "Drop", Role: RoleDestructive,
				Run: func(context.Context, []dev) (Outcome, error) { return Success("gone"), nil }}},
		},
	}
	return App{Pages: Pages{list, details}}
}

func TestValidateAndLowerVisitTheSameNodes(t *testing.T) {
	t.Parallel()

	app := fixture()
	f := app.collectFacts()
	counted := 0
	for _, p := range app.Pages {
		switch pg := p.(type) {
		case Page[listArgs]:
			counted += countValidate(t, pg.Body, f, pg)
		case Page[detailArgs]:
			counted += countValidate(t, pg.Body, f, pg)
		}
	}
	validated := counted

	assert.Equal(t, len(app.validate()), 0)

	l := &appLowerer{entries: map[string]bool{}}
	for _, p := range app.Pages {
		if p.pageNav().Label != "" {
			l.entries[p.pagePath()] = true
		}
	}
	for _, p := range app.Pages {
		p.lowerPage(l)
	}
	assert.Equal(t, l.nodes, validated)
	assert.True(t, validated > 0)
}

func countValidate[A any](t *testing.T, body PageBody, f *facts, p Page[A]) int {
	t.Helper()

	v := &bodyValidator{page: string(p.Path), facts: f}
	body.validateBody(v)
	assert.Equal(t, len(v.errs), 0)
	return v.nodes
}

func TestLowerResolvesShadowLinkAndTabs(t *testing.T) {
	t.Parallel()

	out, err := fixture().lower()
	assert.NoError(t, err)

	details := out.ByPath["/device/{id}"]
	assert.Equal(t, details.Nav.Shadow, "/device")
	assert.True(t, details.Nav.Hidden)
	assert.Equal(t, out.ByPath["/device"].Nav.Label, "Devices")

	tabs := details.Body.(*ir.Tabs)
	assert.Equal(t, tabs.ID, "tabs") // no ID declared: named for its component
	assert.DeepEqual(t, []int(tabs.Tabs[1].Body.(*ir.Stack).Children[0].(*ir.Split).Children[0].Addr()), []int{1, 0, 0})

	table := out.ByPath["/device"].Body.(*ir.Table)
	assert.Equal(t, table.ID, "devices")
	assert.Equal(t, table.PageSize, 2)
	assert.Equal(t, table.RowClick.Dest, "/device/{id}")
	assert.Equal(t, table.Columns[0].Key, "id")
	assert.True(t, table.Bulk[0].Bulk)
	assert.Equal(t, table.Bulk[0].Role, ir.RoleDestructive)
}

func TestLowerTableLoadNormalisesTotal(t *testing.T) {
	t.Parallel()

	out, _ := fixture().lower()
	table := out.ByPath["/device"].Body.(*ir.Table)
	rows, total, err := table.Load(context.Background(), ir.Query{})
	assert.NoError(t, err)
	assert.Equal(t, len(rows), 2)
	assert.Equal(t, total, -1) // Rows.Total left zero: unknown, not "zero rows"
}

func TestBindStartsFromLoadedModelAndReportsEveryFailure(t *testing.T) {
	t.Parallel()

	out, _ := fixture().lower()
	form := out.ByPath["/device/{id}"].Body.(*ir.Tabs).Tabs[0].Body.(*ir.Form)
	base, _ := form.Load(context.Background())

	ipName := form.Fields[0].Group[1].Name
	countName := form.Fields[1].Name

	got, errs := form.Bind(base, map[string]string{ipName: "9.9.9.9", countName: "7"})
	assert.Equal(t, len(errs), 0)
	assert.DeepEqual(t, got, dev{Id: "d1", Ip: "9.9.9.9", Count: 7}) // Id kept: it has no Store

	_, errs = form.Bind(base, map[string]string{ipName: "nope", countName: "x"})
	assert.Equal(t, len(errs), 2)
	assert.Equal(t, errs[0].Label, "IP")
	assert.Equal(t, errs[1].Label, "Count")
	assert.Equal(t, errs[1].Message, "Must be a whole number.")
}

func TestLowerOutcome(t *testing.T) {
	t.Parallel()

	ip := String[dev]{Label: "IP", Load: func(d dev) string { return d.Ip }}
	e, err := lowerOutcome[dev](Reject(Field[dev](Placeholder[dev]{Accessor: ip, Text: "x"}, "taken")))
	assert.NoError(t, err)
	assert.Equal(t, e.Kind, ir.OutcomeReject)
	assert.DeepEqual(t, e.Fields, []ir.FieldError{{Label: "IP", Message: "taken"}})

	// Each constructor lowers to its kind and keeps its message.
	for kind, o := range map[ir.OutcomeKind]Outcome{
		ir.OutcomeSuccess: Success("ok"), ir.OutcomeWarning: Warning("careful"), ir.OutcomeFailure: Failure("no"),
	} {
		got, err := lowerOutcome[dev](o)
		assert.NoError(t, err)
		assert.Equal(t, got.Kind, kind)
	}
	zero, err := lowerOutcome[dev](Outcome{})
	assert.NoError(t, err)
	assert.Equal(t, zero.Kind, ir.OutcomeSuccess) // a quiet success

	_, err = lowerOutcome[dev](Reject(Field[int](Int[int]{Label: "n"}, "bad")))
	assert.Error(t, err) // a Reject about another model
	_, err = lowerOutcome[dev](Success("x").Then(Target{Err: context.Canceled}))
	assert.Error(t, err)

	// Then sends an accepted outcome on and leaves a refused one where it is.
	assert.Equal(t, Success("x").Then(Target{URL: "/a"}).redirect.URL, "/a")
	assert.Equal(t, Warning("x").Then(Target{URL: "/a"}).redirect.URL, "/a")
	assert.Equal(t, Failure("x").Then(Target{URL: "/a"}).redirect.URL, "")
	assert.Equal(t, Reject[dev]().Then(Target{URL: "/a"}).redirect.URL, "")
}
