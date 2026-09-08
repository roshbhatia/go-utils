# go-utils

Shared Go primitives for Roshan's terminal tools.

- `agents`: reads the generated harness registry.
- `animation`: validates and plays renderer-neutral terminal animations.
- `cell`: measures, truncates, and fits ANSI-styled terminal cells.
  `Truncate`, `Fit`, `RightFit`, and `ClipWord` accept an optional tail such as
  `"..."`; omitted tails use the Unicode ellipsis.
- `completion`: generates contextual Bash, Zsh, Fish, and Nushell completions,
  terminal help, and Markdown help from one nested command tree.
  `Command.Synopsis` supplies short help, while `Command.LongDescription`
  supplies detailed help.
  `CompletionCommand` adds newline-delimited runtime candidates to a command
  argument or flag. Shells preserve each line as data, including spaces and
  shell metacharacters. Generated completions suppress unmodeled filesystem
  candidates in every shell. Fish omits records that contain a tab because its
  completion protocol reserves tabs for descriptions.
- `config`: loads typed YAML, applies environment overrides, and emits JSON Schema.
- `diffview`: renders multi-repository diffs as symbol and call trees.
- `git`: runs Git with inherited repository state removed.
- `keymap`: validates key catalogs and generates hints and help rows.
- `panes`: resolves pointer hits and directional focus between pane rectangles.
- `paths`: reads generated XDG path manifests.
- `provider`: discovers and invokes shell-independent external providers.
- `ui`: defines terminal colors, keys, layout, status, and themes.
- `terminal`: detects TTY capabilities and filters terminal control replies.
- `workspace`: finds the active workspace and its repositories.
- `xdg`: resolves strict absolute XDG base directories.

The `ui` package holds renderer-independent theme, status, navigation, and
layout contracts. Bubble Tea, OpenTUI, and plain terminal clients can map these
values into their own rendering APIs.

```go
palette := ui.DefaultPalette()
action, ok := ui.ActionFor(ui.Key{Name: "j"})
frame := ui.ResolveLayout(160, 48, true)
```

The `animation` package implements `terminal.animation/v1`. Each sequence uses
either `fps` or a `duration_ms` on every frame. Frames contain full text and a
semantic style role. Rendering pads all frames to stable display-cell
dimensions. Compact variants are optional. Each animation has one static
reduced-motion frame.

FPS timing uses `ceil(1000 / fps)` milliseconds per frame. Per-frame timing
requires `duration_ms` on every frame. `ping_pong` does not repeat endpoints.
Easing changes the reported progress within a frame, not frame selection.

```go
config, err := animation.ParseYAML(source)
if err != nil {
	return err
}
sequence, ok := config.Select("loading", animation.Preferences{Compact: narrow})
frame := sequence.Render(elapsed.Milliseconds())
```

Generate the checked JSON Schema with
`go run ./internal/cmd/animation-schema`.

Provider manifests use the neutral `provider/v1` contract that
[provider-spec](https://github.com/roshbhatia/provider-spec) publishes.
`provider/spec/` is a copy of one pinned release: its JSON Schema, `VERSION`,
and manifest fixtures. `provider.Schema()` returns that schema byte for byte
and `provider.SpecVersion` names the release. `nix flake check` fails when the
copy differs from the `provider-spec` flake input, and the conformance test
holds `Decode` to the fixtures. To bump the spec, update the input URL, copy
the three paths again, and commit them together.

An action renders each argument and environment value as an independent Go
template. The runtime then executes the declared argv directly and exchanges
bounded JSONL request, event, and result frames over standard input and
output. Durations follow the spec grammar, a strict subset of Go's
`time.ParseDuration`: `1h30m` and `1.5s` parse, `0`, `-1s`, and `.5s` do not.

A manifest's `defaults` may name the provider's models: `model` for a plain
request and `light` for bulk work where the round trip should cost less than
the text it reads. `Manifest.ResolveModel` maps the role words `default` and
`light` onto those ids and passes any other value through as a literal model
id; asking for a light model the provider does not declare is an error, never
a silent fallback to the heavier one.

## Development

```bash
nix develop
go test -race ./...
nix flake check
```
