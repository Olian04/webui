package webui

import (
	"github.com/Olian04/webui/internal/ir"
)

// lower erases the declaration's type parameters into ir: generics gone,
// reflection spent, closures wrapped. Every type assertion in the system is
// created here, so nothing downstream asserts.
//
// Mapping only. validate has already run, so lower decides nothing and reports
// nothing: a missing field or a bad pattern is impossible by the time it gets
// here. It may repeat work — recompiling a Pattern rather than threading the
// compiled value through — but it never repeats a decision. Wanting to return
// an error from lower is the signal that a check belongs in validate.
//
// ERASURE RULE, which decides the shape of everything below. A concrete type
// can be named in a type switch only where all its parameters are in scope:
//
//	Page[A] in a.Pages         A unknown      -> method on pageLike
//	Form[M] / Table[M] in Body M unrelated to A -> method on PageBody
//	Link[M, A] in RowClick[M]  A unknown      -> method on RowClick[M]
//	Accessor[M] inside Form[M] M known        -> type switch
//	Fields[M] inside Action[M] M known        -> type switch
//
// So the leaves of the lowering are methods that live beside their types, and
// the interiors are ordinary switches.
func (a App) lower() (*ir.App, error) {
	// 1. Pages. a.Pages is []pageLike, so A is gone here. Lowering is a method
	//    on the interface, implemented by Page[A]:
	//
	//      lower() *ir.Page
	//
	//    Nav.Shadow resolves from a *Nav to the shadowed page's PathTemplate.
	//    That needs the other pages, so either pass a map[*Nav]string built
	//    first, or resolve in a second sweep once every page is lowered.
	//    BREAK OUT: resolveShadows(pages []*ir.Page, byNav map[*Nav]string).

	// 2. ByPath index, built after the loop. ir.App.ByPath is destination
	//    resolution for Open, not a route table; the router derives its own.

	// 3. Per page, inside Page[A].lower():
	//
	//      a. Args. Walk A's fields once, producing []ir.ArgSpec with the name
	//         from internal/args and InPath from whether Path names it.
	//      b. Decode. Close over the field indices and kinds from (a), so no
	//         reflect.Type survives into the IR and no reflection happens per
	//         request. This is the hot path; resolve everything now.
	//         BREAK OUT: args.Decoder(t reflect.Type, specs []ir.ArgSpec) in
	//         internal/args — it owns parsing, and lower should not.
	//      c. Guard. Wrap func(ctx, A) error as func(ctx, any) error with the
	//         single assertion a.(A).
	//      d. Body, with addr ir.Addr{}.

	// 4. Body tree. Method on PageBody: lower(at ir.Addr) ir.Node.
	//
	//      Stack / Split  -> children lowered with at+[i]
	//      Tabs           -> panels lowered with at+[i]; Label copied
	//      Form[M]        -> *ir.Form  (step 5)
	//      Table[M]       -> *ir.Table (step 6)
	//
	//    Addr is assigned here and nowhere else. It is the leaf identity a
	//    refresh request names, so it must be derived from position, never
	//    from anything the user typed.

	// 5. Form[M].lower(at):
	//
	//      Load   wrap func(ctx) (M, error) as func(ctx) (any, error)
	//      Fields lower each Accessor[M] (step 7)
	//      Submit lower Action[M] (step 8)
	//      Bind   BREAK OUT: bindFunc[M](fields []Accessor[M]) — build once,
	//             close over the per-field parse and Store. It applies every
	//             value it can and returns every failure, so one round trip
	//             shows the user all of them. Parse failures short-circuit the
	//             rules for that field only, not for the form.

	// 6. Table[M].lower(at):
	//
	//      Load     wrap func(ctx) ([]M, error) as func(ctx) ([]any, int, error).
	//               Total is -1 until the paging signature lands; the IR field
	//               exists so adding it later is not a break.
	//      Columns  lower each Accessor[M] (step 7)
	//      RowClick method on RowClick[M]: Link[M, A] -> *ir.Link with Dest
	//               from Page.Path and Args wrapping func(ctx, M) A into
	//               func(ctx, any) map[string]string; Action[M] ->
	//               *ir.ActionTarget.
	//      Actions / BulkActions lower each (step 8)

	// 7. Accessor[M] -> ir.Field. M is known, so a plain type switch:
	//
	//      String[M]      Get formats, Set parses then assigns via Store.
	//                     Set is nil when Store is nil — that is what
	//                     "no Store means read-only" lowers to.
	//      Int / Float    same, with strconv in the Set wrapper.
	//      Group[M]       ir.Field with Group filled, Get and Set nil.
	//      Sortable       lower the inner accessor, then set SortKey.
	//      Placeholder    lower the inner accessor, then set Placeholder.
	//
	//    Decorators flatten: they are a declaration convenience and have no
	//    runtime existence.
	//
	//    BREAK OUT: lowerAccessor[M](acc Accessor[M]) ir.Field. Recursive for
	//    Group and the decorators, and the one place Store becomes Set.
	//
	//    Rules map to the single ir.Rules struct. internal/rules owns the RE2
	//    compile and the constraint check; lower only copies the data across.

	// 8. Action[M] -> *ir.Action:
	//
	//      Guard  wrap func(ctx, M) error as func(ctx, any) error
	//      Run    wrap func(ctx, M) (Effect, error) as
	//             func(ctx, any) (ir.Effect, error), converting the Effect:
	//               Toast     copied
	//               Redirect  Target.URL, already resolved by Open, which had
	//                         ctx and therefore the prefix. Effect is a
	//                         request-time value, not part of the declaration,
	//                         so a URL here is not a leak.
	//               Fields    FieldErrors is Fields[M]; M is known inside this
	//                         wrapper, so type switch it and resolve each
	//                         Accessor to its Label.
	//      Bulk   true for BulkActions, where the subject is []M.
	//
	//    BREAK OUT: lowerEffect[M](e Effect) ir.Effect.
	return nil, nil // TODO: implement
}
