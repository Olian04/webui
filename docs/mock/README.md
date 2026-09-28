# webui — UI mock, as static files

## Quick Start

```sh
cd webui-mock
python3 -m http.server 5432
open http://localhost:5432/
```

> **Serve from inside _this_ folder.**  
> All links are root-absolute (`/index.html`, `/assets/app.css`), so serving the parent directory will not work.

---

## What Is Real Here

Every page is a real HTML document on disk, and every link points at an actual file.  
Navigation is genuine cross-document navigation.

The cross-fade between pages is done with four lines of CSS in `assets/app.css`:

```css
@view-transition { navigation: auto; }
.sidebar { view-transition-name: wui-sidebar; }
.topbar  { view-transition-name: wui-topbar; }
.scroll  { view-transition-name: wui-content; }
```

No script is involved. **Turn JavaScript off in the browser:**  
- You can navigate every page, open any device, read any table.
- The sidebar still never appears to move.

Animates in Chrome 126+ and Safari 18.2+.  
Firefox (as of writing) has same-document view transitions; cross-document form is pending.  
All other browsers default to ordinary navigation.

---

## Path vs Query Arguments

- **Device ID** is a path argument, making each device a real file:
  ```
  /device/dev_27c38b.html
  ```

- **Sort, filter, paging, tab** are query arguments – multiple views on the same file:
  ```
  /index.html?sort=ip&dir=asc&offset=10
  ```

_This is `ArgSpec.InPath` made literal by the filesystem—row links work with no script at all._

---

## The One Seam

A static file server cannot vary its response by query string.  
So, query arguments are **applied by `assets/render.mjs`, in the browser, on load**.

That script stands in for the **server**, not for a client framework. It:
- Decodes arguments
- Filters and sorts rows
- Renders HTML

_This matches what the Go runtime would do, once, on the way out. In the real app this file does not exist._

**With JavaScript fully disabled,** you get each page in its default view.  
All navigation still works, but query arguments are ignored because there is nothing honoring them.

---

## The One Genuine Enhancement

`assets/enhance.mjs` is the only authentic client-side script.  
It does **not** change what the links are—only how much of the page is replaced when a link is followed.

#### The rule it applies:

```
a link to a DIFFERENT path  -> an ordinary navigation, cross-faded by CSS
a link to the SAME path, with different arguments, inside a [data-leaf]
                            -> only that leaf is replaced
```

Turn "Panel refresh" _off_ in the sidebar to disable it.  
Nothing breaks; links become ordinary navigations.

_This is the entire progressive-enhancement claim, demonstrable with a switch._

---

## Layout

| File / Asset              | View / Role       | Arguments                              |
|-------------------------- |------------------|----------------------------------------|
| `index.html`              | Devices          | site, status, q, sort, dir, offset     |
| `device/{id}.html`        | Device           | tab  _(37 files)_                      |
| `alert.html`              | Alerts           | sev, state                             |
| `settings.html`           | Ingest           | none                                   |
| `retention.html`          | Retention        | none                                   |
| `forbidden.html`          | Guard rejection  |                                        |
| `compile-error.html`      | Compile failure  |                                        |

| Static Asset         | Purpose                                                  |
|----------------------|---------------------------------------------------------|
| `assets/app.css`     | The design language. Every value is a token.            |
| `assets/data.mjs`    | The dataset. Deterministic, so pages agree.             |
| `assets/views.mjs`   | All markup, one implementation.                         |
| `assets/render.mjs`  | Stands in for the server (query arguments).             |
| `assets/enhance.mjs` | The (optional) panel-level refresh.                     |
| `assets/prefs.js`    | Theme & role, pre-paint, from localStorage.             |
| `build.mjs`          | Regenerates every page: `node build.mjs`                |

`views.mjs` is imported by both `build.mjs` and in the browser, so static files and client-side re-render cannot disagree.

---

## Known Divergences from `docs/design.md`

- **Bulk action bar** is always present (does not appear on selection).  
  Without script, there is no selection event, and a form post is the honest no-JS shape for a bulk action.

- **Role gating** uses a `[data-role]` attribute on `<html>` (driving CSS), not a rendered `disabled`.  
  A server would emit the attribute per control.
