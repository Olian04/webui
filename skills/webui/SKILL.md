---
name: webui
description: >
  Build admin panels, dashboards and internal tools in Go with the webui library
  (github.com/Olian04/webui). Use when the task is to create, extend, debug or review a
  UI made with webui: pages, tables, forms, actions, navigation, guards, theming, or
  mounting it in an http server. Also use whenever go.mod requires
  github.com/Olian04/webui or the user mentions webui.
---

# Building a UI with webui

webui turns a declaration written in Go into a working admin UI. You describe pages,
tables and forms as plain Go values; the library validates the declaration, serves
server-rendered HTML, and owns everything else. You never write HTML, CSS or
JavaScript.

This skill is the **method and the judgement**. It deliberately does not list the API.
The API changes between versions, and a list here would be wrong the day it changes.
Read the documentation for the version in use, as step 0 says, and trust it over this
file and over your memory.

## Step 0: read the docs for the version in use

Do this before writing any webui code, every session, and again after the version
changes. Never write webui code from memory.

1. Find the version in use: `grep Olian04/webui go.mod`. If the module is not required
   yet, add it with `go get github.com/Olian04/webui@latest` and say which version.
2. Read the package documentation for exactly that version. The most reliable way is
   the module cache, which holds precisely the version go.mod pins:

   ```sh
   go doc -all github.com/Olian04/webui/pkg/webui            # everything
   go doc github.com/Olian04/webui/pkg/webui Table           # one type, with its fields
   go doc github.com/Olian04/webui/pkg/webui Page.Path       # one field or method
   ```

   If the module is not downloaded yet, run `go mod download github.com/Olian04/webui`
   first.

3. The same documentation is on pkg.go.dev, with runnable examples that are tested
   against the code. Pin the version in the address:
   `https://pkg.go.dev/github.com/Olian04/webui@<version>/pkg/webui`, or without
   `@<version>` for the latest. The package overview and the Examples are the best
   place to learn the shape of an app.
4. The repository's `cmd/demo` is a complete reference app that uses most of the
   library. Read it at the matching tag:
   `https://github.com/Olian04/webui/tree/<version>/cmd/demo`.

**If this file and the documentation disagree, the documentation is right.** If a field,
type or behaviour is not in the documentation for the version in use, it does not
exist: do not invent it, and do not guess a name that "should" be there. See "When the
library cannot do it" below.

## The mental model

These ideas are stable across versions. They explain why the library looks the way it
does, and they let you design before you look up names.

- **A declaration, compiled once.** An app is a value built from pages, which hold
  bodies made of tables and forms. Compiling it checks the whole declaration at once and
  returns an `http.Handler`. Everything wrong is reported together, each problem with
  where it is and how to fix it. Compile at startup so a mistake fails the process, not
  a visitor.
- **Declare it as static configuration.** Write accessors, pages and the app as
  package-level `var`s, in files grouped by area. The declaration should read like a
  configuration file, with logic only where data is read or written.
- **One accessor, two renderings.** A field of a model is described once, by an
  accessor, and the same value is a column in a table and an input in a form. Whether
  it is editable is decided by whether you gave it a way to store a value. Different
  kinds of value (text, numbers, status, moments, links and so on) have different
  accessors; the documentation lists the current ones.
- **The address holds the view.** Sorting, filtering, paging, the selected tab and
  the page's arguments live in the URL, so every view can be linked, bookmarked and
  reloaded. The library owns those parameters. Your argument struct is only for
  your own page arguments.
- **Types connect pages.** A page is typed by its argument struct, and links between
  pages are checked by the compiler. Where two pages link to each other, the
  documentation describes how to name a page without a Go initialization cycle.
- **Works without JavaScript.** Every control is a real link or form. A small script
  only adds speed. Do not design anything that needs script, and do not add your own.
- **Actions say how they ended, not what to do about it.** An action reports success,
  warning, failure or a rejection of particular fields, and the library decides the
  message, the colour and where the user goes. Say where to go next only when you must.
- **Gating is one rule, in one place.** A guard decides whether a visitor may open a
  page or run an action. The same check hides or disables the control and refuses the
  request, so there is no second rule to keep in step.

## How to build an app

Work in this order. Each step is small and can be compiled and looked at before the
next.

1. **Model.** Plain Go structs for what the UI shows. webui does not care where the
   data lives, so keep storage behind your own functions.
2. **Accessors.** One per field you will show or edit, written once and reused as both
   column and form field. Give every accessor a clear, unique label: labels name
   columns and tabs, and appear in addresses, so renaming one changes links.
3. **Leaves.** A table for lists and a form for one record. A form with nothing to
   submit is the way to show one record read-only. For a table, choose where the rows
   come from by how many there are and whether the source can order and filter them;
   the documentation for the table says which option fits which size, and what the
   library does for you in each.
4. **Actions.** Mutations are actions with a guard and a way of reporting how they
   ended. Keep them thin: call your own functions, return an outcome.
5. **Pages.** A path (with arguments in it where a page is about one thing), a
   navigation entry only where the page belongs in the menu, an optional guard, and a
   body. Detail pages usually have no entry of their own; the library works out which
   entry to light.
6. **The app.** Pages, the brand, an optional theme, and the menu of external links.
7. **Mount it.** Compile with the prefix the app lives under, and mount the handler
   there, behind your own authentication. See "Authentication" below.
8. **Look at it.** Run it and open it in a browser, in light and dark, as each kind of
   visitor. Types and tests do not show layout.

## Authentication and security

- **webui does no authentication.** Put your own middleware in front of the handler,
  and put whatever identity you need in the request's context. Guards receive that
  context, so a guard reads the identity from it.
- **Guards run on every request, including posts.** Hiding a control is a convenience;
  the guard is what protects. The guard's message is shown to the visitor, so write it
  for them, and keep it free of anything they should not see.
- **Errors from your data code are not shown.** A failed load shows a generic message
  and logs the cause. Return an error for a real failure and an empty result for "none".
- The library already sets the security headers, refuses cross-site posts and escapes
  every value it renders. Do not work around any of it. Do not put untrusted text in
  places the documentation does not describe.

## Conventions that keep an app good

- Keep the declaration (pages, accessors) apart from the data layer; the declaration
  calls your functions and never the other way round.
- One file per area of the product, each declaring its pages and the accessors they
  use. Share accessors by reusing the variable.
- Prefer many small pages to one page that does everything. Pages are cheap, links are
  checked, and each page can have its own guard.
- Use the library's own mechanisms before inventing yours: its way to return to the
  page a form was opened from, to link between pages, to report outcomes. If you are
  writing redirect or flash logic yourself, you have missed one.
- Let the library validate. Declare constraints as data on the accessor and do not
  duplicate them in the action; only rules that need your data (such as uniqueness)
  belong in the action, reported as a rejection of the field.

## Checking your work

- `go build ./...` and run the app. A compile problem is served as a page at every
  address, and returned as an error from the compile call; read each problem's fix.
- Write tests against the compiled handler with `net/http/httptest`: request a page,
  post to an action, and look at the status, the redirect and the HTML. The runnable
  examples in the documentation show the pattern.
- Check the page as a visitor who may not do things: guarded controls, hidden entries,
  forbidden pages.
- After upgrading webui, read the release notes for the version, if there are any, then
  re-read the documentation and recompile. A compile error after an upgrade is usually
  an API that moved: the documentation says where it went.

## When the library cannot do it

The library is deliberately small, and it does not let you write markup. Before
concluding that something is impossible, search the documentation for the concept, not
for the name you would give it: it may exist under another word, or be done for you.

If it genuinely cannot be done:

1. Say so plainly to the user, and say what the closest thing is.
2. Prefer a design that fits the library, such as splitting one page into two, to a
   workaround that fights it. Do not inject markup or script, and do not post-process
   the handler's output.
3. If it is a real gap, suggest the user open an issue at
   <https://github.com/Olian04/webui/issues> with the use case, rather than building a
   fragile local patch.

## Quick reference

- Module: `github.com/Olian04/webui`; the package to import is `.../pkg/webui`.
- Docs for the pinned version: `go doc -all github.com/Olian04/webui/pkg/webui`.
- Docs and runnable examples: <https://pkg.go.dev/github.com/Olian04/webui/pkg/webui>.
- Reference app: `cmd/demo` in the repository, at the tag of the version in use.
- Issues and releases: <https://github.com/Olian04/webui>.
