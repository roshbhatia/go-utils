# go-utils

Shared Go primitives for Roshan's terminal tools.

- `agents`: reads the generated harness registry.
- `animation`: validates and plays renderer-neutral terminal animations.
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
- `paths`: reads generated XDG path manifests.
- `provider`: discovers and invokes shell-independent external providers.
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

Provider manifests use the neutral `provider/v1` contract. An action renders
each argument and environment value as an independent Go template. The runtime
then executes the declared argv directly and exchanges bounded JSONL request,
event, and result frames over standard input and output.

## Development

```bash
nix develop
go test -race ./...
nix flake check
```
