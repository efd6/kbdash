# kbdash

Describe Kibana dashboards from Elastic integration packages.

Kibana dashboard JSON files are opaque saved-object exports with no
published spec and no official tooling for inspection. kbdash parses
them and produces either a human-readable text summary or a Graphviz
wireframe showing the dashboard layout, panel types, and field inputs.

## Install

```
go install github.com/efd6/kbdash@latest
```

For the `-xdot` flag you also need Graphviz and python3-xdot:

```
sudo apt install graphviz xdot
```

## Usage

```
kbdash [flags] <path> [path ...]
```

Each path is a package directory (containing `kibana/dashboard/`) or an
individual dashboard JSON file.

### Text output

```
kbdash packages/trend_micro_vision_one
```

Lists each dashboard with its controls, global query/filters, and panels
sorted by grid position. For each panel: title, type, visualization
subtype, source fields with aggregation operations, filters, and links.

```
=== [Logs TrendAI Vision One] Audit ===
File: trend_micro_vision_one-02296130-0c1b-11ed-8d26-77f06c571b89.json
Description: TrendAI Vision One Audit Events Overview.

Global query: data_stream.dataset : "trend_micro_vision_one.audit"
Global filters: (none)

Panels (48-column grid, sorted by position):

  [0,0 24x15] "Distribution of Audit by Result" (lens: lnsPie)
    Fields: trend_micro_vision_one.audit.result (terms)

  [24,0 24x15] "Distribution of Audit by Access Type" (lens: lnsPie)
    Fields: trend_micro_vision_one.audit.access_type (terms)
```

### Graphviz wireframe

Render to SVG:

```
kbdash -dot packages/trend_micro_vision_one | neato -n -Tsvg -o wireframe.svg
```

Interactive viewer:

```
kbdash -xdot packages/trend_micro_vision_one
```

The wireframe preserves the Kibana 48-column grid layout. Each panel is
a box at its actual grid position and size, labelled with the panel
title, type, and fields. Panels are colour-coded by type:

| Colour | Panel type |
|--------|-----------|
| White | Lens visualisation |
| Yellow | Navigation links |
| Green | Legacy visualisation (markdown, etc.) |
| Blue | Saved search |
| Orange | Map |

When multiple dashboards are rendered, each gets a labelled bounding
frame and they stack vertically. Cross-dashboard navigation edges show
how link panels connect dashboards. Unresolved links (when rendering a
subset of a package's dashboards) appear as dashed ellipses.

### Write to file

```
kbdash -o description.txt packages/trend_micro_vision_one
kbdash -dot -o wireframe.dot packages/trend_micro_vision_one
```

## Supported panel types

- **lens**: visualisation type, series type, layer columns (field +
  operation), and per-panel filters. Handles both `formBased` and
  `indexpattern` datasource keys across Kibana versions.
- **links**: dashboard navigation links with resolved destination IDs.
- **visualization**: legacy visualisations including markdown content.
- **search**: saved search references (definition is not inlined in
  the dashboard JSON, so only the reference ID is shown).
- **map**: geo fields and source types from layer descriptors.

Unknown panel types are passed through with a warning rather than
silently dropped.

## Compatibility

Parses dashboard JSON across Kibana versions (tested against packages
with `typeMigrationVersion` from 8.x through 10.x). Handles
string-encoded JSON fields (`panelsJSON`, `searchSourceJSON`,
`controlGroupInput`) that appear in some older exports.
