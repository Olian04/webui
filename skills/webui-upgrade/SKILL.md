---
name: webui-upgrade
description: >
  Upgrade or downgrade the webui library (github.com/Olian04/webui) in a Go project,
  between any two versions, and fix what the change breaks. Use when the user asks to
  upgrade, bump, update, pin or downgrade webui, to move to a newer or a specific
  version, or when a build or webui's Compile broke after the version in go.mod changed.
  Reads the migration guides that ship with the stable releases, and falls back to the
  release notes and the documented API when a version has none.
---

# Upgrading webui in a project

This skill moves a project from one version of webui to another, in either direction,
and brings the project's code along. It is the **method**. It deliberately does not say
what changed in any version: that is in the migration guides and the release notes of
the versions involved, and a list here would be wrong the day a version is released.
Read what the versions themselves say, as the steps below do, and trust it over this file
and over your memory.

To write new webui code, use the `webui` skill, which also says how to read the
documentation for a version. This one is for changing the version.

## Step 0: say what is being upgraded, and prepare

1. **From:** the version in use. `grep Olian04/webui go.mod`. If the module is not
   required, there is nothing to upgrade: this is a new dependency, and the `webui` skill
   is the one to use.
2. **To:** the version the user asked for. List what exists:

   ```sh
   go list -m -versions github.com/Olian04/webui
   ```

   A version is **stable** when it is `vX.Y.Z` and a **prerelease** when it is `vX.Y.Z-N`
   (the Nth prerelease before `vX.Y.Z`). If the user said "latest" or "newest", take the
   newest stable version that is newer than the one in use; if there is none, or the project is on
   a prerelease and no stable version is newer, take the newest prerelease. Say which you took and
   why. If a choice is not obvious (a prerelease where a stable version exists, or the other way
   round), ask.
3. **Direction.** The target may be older than the version in use: a downgrade is the same
   work in reverse (see Step 3).
4. **The steps between.** The releases `v` with `from < v <= to` for an upgrade, in that order;
   for a downgrade, `to <= v < from`, newest first. Write them down. They are what Step 1 reads.
5. **A clean start.** `git status` should be clean, so the change is one diff. Work on a
   branch. Do not commit unless the user asked you to.

Say all of this in two or three sentences before changing anything: from, to, direction,
and how many releases lie between.

## Step 1: collect what each release says about breaking changes

Do this for every release in the range, before editing code.

**1. The migration guide.** A **stable** release that breaks the previous stable release
ships a guide, `docs/migrations/<version>.md`, in its source. The module cache has that
source, so download the version and read it:

```sh
go mod download github.com/Olian04/webui@<version>
ls "$(go list -m -f '{{.Dir}}' github.com/Olian04/webui@<version>)/docs/migrations/"
```

A guide covers **everything since the previous stable release**, and not only the one
before it. So read the guide of each stable release in the range, in order, and no
others: the guides do not repeat each other. A stable release with no guide file had no
breaking change since the previous stable one, and a project that is already on that
previous stable version needs nothing from it.

**2. When there is no guide.** Prereleases never have one, the first stable release has none
for what changed during the prereleases, and a release that is older than the convention has none.
Use these, in this order, until the breaking changes of every release in the range are known:

- **The release notes and the changelog**, on the releases page,
  `https://github.com/Olian04/webui/releases`, or with `gh release view <tag> --repo
  Olian04/webui`. An entry whose type has a `!` (`feat!: …`) is a breaking change, and
  names its pull request, whose description has a **Breaking** section that says what to do
  and shows a before and after.
- **The difference in the documented API** between the two versions. The documentation is
  the library's own statement of its API, and it can be diffed:

  ```sh
  go mod download github.com/Olian04/webui@<from> github.com/Olian04/webui@<to>
  for v in <from> <to>; do
    (cd "$(go list -m -f '{{.Dir}}' github.com/Olian04/webui@$v)" && go doc -all ./pkg/webui) > "/tmp/webui-$v.txt"
  done
  diff -u "/tmp/webui-<from>.txt" "/tmp/webui-<to>.txt"
  ```

  A removed or retyped name in the diff is a break. A new name is not, but it may be the
  replacement for one that went.
  This is also how to cover the part of a range that lies before the first stable release:
  a project that goes from a prerelease to a later stable release has the guides of the
  stable releases after the first, and these for everything before.
- **The compiler and the library's own checks**, in Step 3: the build says what no longer
  exists, and `Compile` says what the declaration may no longer be. They are the final
  authority, and what the notes missed, they find.

**3. A stable release with a breaking change and no guide is a gap in the library.** Say so
in the report, use the fallbacks above, and suggest that the user report it to the project.

## Step 2: find what applies to this project

A breaking change only matters where the project uses what changed.

1. For each breaking change, search the project for the names it mentions (`grep -rn`), and
   list the files and the lines. A change that touches nothing the project uses is
   noted and left alone.
2. **Behaviour changes** are the ones that are easy to miss: they do not stop the build. A
   guide or a note that says a default, a rule, an address or the way something looks has
   changed is a change to check by looking, so write each one down with where to look.
3. Order the work as the guides do: what stops the build first, then what `Compile`
   refuses, then behaviour.

## Step 3: upgrade

**Go in hops when the range is large.** For a jump over several stable releases, move to
each stable release in turn, and to the target last, building after each: a hop's errors are
the ones its guide describes, and a failure is easy to place. A range inside one prerelease
series, or whose releases have no guides, may be done in one move: the compiler is the
guide there.

For each hop:

1. Change the version and tidy:

   ```sh
   go get github.com/Olian04/webui@<version>
   go mod tidy
   ```

2. Apply the guide for that release in the order it gives, the mechanical changes (a
   rename, a changed signature, a type that gained or lost a parameter) first.
3. Build (`go build ./...`) and fix what the guide did not predict. For every error that is
   not explained, read the documentation for the new version, as the `webui` skill's Step 0
   says: `go doc github.com/Olian04/webui/pkg/webui <Name>` for a name.
4. Run the app's `Compile` and its tests (Step 4), and fix what they say before going on.

**Rules for the edits:**

- Keep what the project does. A migration changes how the project says something to the
  library, not what the app does.
- Make the smallest change that migrates. Do not tidy, restructure or rename what the change
  did not touch.
- Never delete a feature, a column, a check or a test to make the build pass.
- Do not leave a shim or a `TODO` that says "migrate later". Either migrate it, or stop and
  say what is left.
- Update the project's tests to the new API as part of the migration. A test that was
  deleted instead of updated is a loss of the evidence that the migration worked.
- Do not edit generated files; change what generates them.

**If a change needs a decision** (a feature that the library removed, two replacements that
both fit, a behaviour the project depended on), do not guess. Stop, and give the user the
options the guide gives, with the one you would take.

**A downgrade** is the same in reverse: read the guides of the releases being left, newest
first, and undo their changes. What the project uses that the older version does not have
has to be removed or replaced, and a guide cannot say how, since it was written for the
other direction, so expect more decisions and say so before you start.

## Step 4: check

1. `go build ./...`, `go vet ./...` and `go test ./...` pass.
2. **Start the app.** webui checks the declaration when the app starts (`Compile`), and a
   failure is a list of errors, each with a fix. An app that builds can still refuse to
   start, and this is where a rule that the new version enforces shows up. Fix what it says
   in the declaration, not by silencing it.
3. If you can open the app, open the pages that a behaviour note in Step 2 named, and look.
   If you cannot, say that you did not.

## Step 5: report

Tell the user, briefly and in this order:

- **From, to, and the releases between.**
- **Which guides you read, and for which releases there was none** (and what you used instead).
- **What you changed**, grouped by the breaking change that made you change it.
- **What needs a human eye**: behaviour changes, decisions you took, anything you were not
  sure of.
- **What you did not do or could not check**, with the reason. Do not say that something was
  verified if it was not run.

## Quick reference

```sh
go list -m -versions github.com/Olian04/webui                    # what exists
grep Olian04/webui go.mod                                        # what is in use
go mod download github.com/Olian04/webui@<version>               # put a version in the module cache
go list -m -f '{{.Dir}}' github.com/Olian04/webui@<version>      # where it is (read its docs/migrations/)
go get github.com/Olian04/webui@<version> && go mod tidy         # move to it
```

Release notes: `https://github.com/Olian04/webui/releases`. Documentation for a version:
`https://pkg.go.dev/github.com/Olian04/webui@<version>/pkg/webui`.
