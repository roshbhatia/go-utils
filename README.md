# go-utils

Shared Go primitives for Roshan's terminal tools.

- `agents`: reads the generated harness registry.
- `diffview`: renders multi-repository diffs as symbol and call trees.
- `git`: runs Git with inherited repository state removed.
- `paths`: reads generated XDG path manifests.
- `ui`: defines terminal colors, keys, layout, status, and themes.
- `workspace`: finds the active workspace and its repositories.

The `ui` package holds renderer-independent theme, status, navigation, and
layout contracts. Bubble Tea, OpenTUI, and plain terminal clients can map these
values into their own rendering APIs.

```go
palette := ui.DefaultPalette()
action, ok := ui.ActionFor(ui.Key{Name: "j"})
frame := ui.ResolveLayout(160, 48, true)
```

## Development

```bash
nix develop
go test -race ./...
nix flake check
```
