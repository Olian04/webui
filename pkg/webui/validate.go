package webui

// Validates that which cannot be expressed by the type system.
// An optimal validation function should reduce the set of possible runtime errors to zero.
//
// Checks only: validate emits nothing and decides nothing about shape. It also
// never stops at the first problem — Compile reports every error at once so one
// fix cycle clears them all.
//
// Name derivation and RE2 compilation are not reimplemented here. validate
// calls internal/args and internal/rules, so it cannot disagree with lower
// about what an argument is called or whether a pattern is valid.
func (a App) validate() CompileErrors {
	// 1. App facts, gathered before any per-page check, because the cross-page
	//    checks in step 3 need the whole set.
	//
	//      paths   map[string]int  // Path -> how many pages declare it
	//      navs    map[*Nav]bool   // every &Page.Nav, for Shadow resolution
	//
	//    BREAK OUT: collectFacts(a) facts. Pure, small, and the only place that
	//    knows what "the set of pages" means.

	// 2. Per page. Needs A for the argument struct, which App does not have —
	//    a.Pages is []pageLike. So this is a method on the pageLike interface,
	//    implemented by Page[A], where A is still in scope:
	//
	//      validate(facts) []CompileError
	//
	//    Each page checks, in order:
	//
	//      a. A is a struct kind. If not, stop checking this page: every later
	//         check reads its fields.
	//      b. Every field is a supported scalar (string, bool, int, int64,
	//         float64). A URL carries one value per name, so nothing composite.
	//      c. No two fields map to the same argument name after `webui` tags.
	//         Ask internal/args for the name; do not lower-case it here.
	//      d. Every {placeholder} in Path has a field. Report the missing field
	//         name in the Fix, since that is the edit the user has to make.
	//      e. Nav.Shadow, when set, is in facts.navs. A shadow pointing at a Nav
	//         no page owns highlights nothing, silently.
	//      f. Body is non-nil.
	//      g. Body tree, recursively (step 2b).
	//
	//    BREAK OUT: validateArgs(t reflect.Type, path string) []CompileError
	//    covers a-d and is the half that is pure reflection over A.

	// 2b. Body tree. PageBody holds Form[M] and Table[M] for an M unrelated to
	//     A, so the leaves cannot be named in a type switch here. Layouts are
	//     non-generic and can be. So: a validate method on PageBody, with the
	//     layouts recursing into children.
	//
	//     Leaves check:
	//
	//       Form[M]   - Load non-nil.
	//                 - Accessor Labels unique within this form. A FieldError
	//                   resolves by Label, so duplicates make it ambiguous.
	//                 - Recurse into Group.
	//       Table[M]  - Load non-nil.
	//                 - No Group in Columns: a nested group has no meaning in a
	//                   cell, and rejecting beats defining flattening.
	//                 - RowClick, if a Link, targets a path in facts.paths.
	//                   This is the check the declared Link.Page bought.
	//
	//     Inside Form[M] and Table[M], M is known, so accessors ARE type
	//     switchable: case String[M], case Int[M], case Group[M], and so on.
	//
	//     BREAK OUT: validateAccessor(acc Accessor[M], seen map[string]bool)
	//     []CompileError. Shared by Form and Table; the caller decides whether
	//     Group is allowed.

	// 3. Per accessor, inside step 2b.
	//
	//      - Label non-empty.
	//      - Rules match the value kind. StringRules on a String, NumberRules
	//        on Int and Float; the types already enforce this, so the check is
	//        for the decorators.
	//      - Pattern.Expr compiles. Call internal/rules; Go's RE2 rejects the
	//        lookahead and backreference syntax people copy from JavaScript,
	//        and the message should say RE2 rather than "invalid regex".
	//      - Sortable and Placeholder do not wrap a Group.

	// 4. Cross-page, once every page has been visited.
	//
	//      - No duplicate Path. Two pages on one path means the router picks
	//        one and the other is dead.
	//      - Every Link target seen in step 2b is in facts.paths. Collect the
	//        targets during the walk rather than walking twice.

	// NOTE: validate and lower walk the same tree. They can drift — a check on
	// a node lower never reaches, or a node lower emits that validate never
	// saw. A test that asserts both visit the same node count for a fixture app
	// is cheaper than abstracting the traversal.
	return nil
}
