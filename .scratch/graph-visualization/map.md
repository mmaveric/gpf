# Generate a graph visualization

## Destination

Provide a Go API that writes a built graph's logical topology as Graphviz DOT, so callers can render and inspect the graph. A prototype compared logical and execution-expanded forms; the user selected the logical view.

## Decisions so far

- [Generate a logical graph visualization](issues/01-prototype-graph-visualization.md): prototype both candidate views, then expose the selected logical topology through `Definition.WriteDOT`.

## Out of scope

- Execution-expanded graph visualization is not included in the public API.
- A CLI, interactive UI, configurable styling, or runtime-state visualization.
