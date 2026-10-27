# Generate a logical graph visualization

Type: task
Mode: HITL
Status: resolved
Blocked by: none

## Problem Statement

Users can build and validate a graph, but currently lack a visual way to inspect its topology. It is not yet clear whether the most useful visualization is the graph as authored or the execution-expanded graph (which may include replicas or pooled nodes).

## Solution

Create a prototype based on the existing small graph example. Add a replicated group, build the definition, generate DOT for both the logical and execution-expanded representations, and render both as SVG. The user will inspect the images and choose which representation should be supported by the production API. Once selected, implement the selected view through the public `Definition` API.

The production API is a method on `Definition` that writes DOT to an `io.Writer`. The logical representation is selected; execution-expanded visualization remains a prototype comparison only.

## User Stories

1. As a graph author, I want to see the topology I built as a diagram, so that I can understand its nodes and connections.
2. As a graph author, I want to see both the logical and execution-expanded forms during the prototype, so that I can judge which view is more useful.
3. As a graph author, I want nodes identified by type and ID, so that nodes of the same type are distinguishable.
4. As a graph author, I want directed connections and their ports shown, so that I can follow data flow through the graph.
5. As a graph author, I want edge kinds distinguished when the graph model exposes them, so that structurally different connections are apparent.
6. As a graph author, I want isolated nodes included, so that the diagram represents the complete built graph rather than only connected components.
7. As a Go library caller, I want the eventual API to write DOT to an `io.Writer`, so that I can choose whether to save or further process the output.
8. As a Go library caller, I want writer errors returned to me, so that failed output is not silently treated as successful.
9. As a maintainer, I want deterministic output where practical, so that diagrams and tests are stable and useful in diffs.

## Implementation Decisions

- Use the public `Definition` API as the prototype and test seam: create a graph through the existing Builder API, then inspect the rendered output through the resulting built definition.
- Start from the existing two-node, one-edge build example and add a replicated node if existing graph APIs allow a compact scenario. Replication is sufficient to expose a logical-versus-expanded difference; a pool is not required.
- Produce DOT descriptions for both logical and execution-expanded representations and render them to SVG for manual comparison. Graphviz is available in the current development environment; production DOT generation must not depend on invoking Graphviz.
- Include node type and ID, directed connections, port labels, edge kind when available, and isolated nodes. Keep output stable where practical.
- The production API seam is a `Definition` method writing to `io.Writer`; return write errors to the caller.
- The user selected the logical representation after reviewing the SVGs. The production method visualizes only the authored topology.
- Keep CLI and visualization styling options out of scope.
- Node labels use the available registration kind (`stateful`, `closure`, or `ingress`) and ID. Concrete Go type names are not stored in graph metadata.
- `Definition.WriteDOT` returns invalid-definition and invalid-writer errors, propagates write failures, and reports short writes.

## Testing Decisions

- Test externally observable DOT output by building a graph through the public Builder API and checking that generated output contains all expected nodes, directionality, port labels, and edge distinctions. Include an isolated node and a replicated-node scenario where supported.
- Verify repeated generation for the same definition is stable, write errors and short writes are reported, and invalid inputs are rejected.
- Test error-edge labeling as well as data edges through a built definition. The logical and execution-expanded fixture comparison was reviewed manually before implementation.
- Prefer the existing graph package build examples and graph definition tests as prior art. Tests should assert graph semantics rather than internal representation details.

## Out of Scope

- Rendering the execution-expanded graph in the production API.
- A command-line interface, interactive UI, or Graphviz dependency in production code.
- Styling configuration, runtime state such as queues/workers/CPU placement, or visualization of a graph before it is built.
- A generalized renderer abstraction or support for image formats beyond the prototype SVG previews.

## Further Notes

- Existing graph build example has two nodes and one edge, but no replication. Use it as the base, not as the final comparison fixture.
- The prototype comparison is available in the sibling directory: `logical.svg` and `execution-expanded.svg` (PNG previews and DOT source are also included). The production renderer is `Definition.WriteDOT`.
- The fixture labels nodes semantically (Source, Transform, Sink, Isolated). Current graph metadata records registration kind and port counts, but not concrete Go type names; the prototype does not resolve the public API design for that limitation.
- The renderer labels nodes by registration kind and ID because current graph metadata does not retain concrete Go type names.
- The temporary graph-package test used internal definition metadata to render the execution-expanded comparison. Production rendering uses the public `Definition.WriteDOT` method for the selected logical view.
