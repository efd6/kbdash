package main

import (
	"fmt"
	"io"
	"strings"
)

func describeText(w io.Writer, dashboards []DashboardInfo) {
	for i, d := range dashboards {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "=== %s ===\n", d.Title)
		fmt.Fprintf(w, "File: %s\n", d.File)
		if d.Description != "" {
			fmt.Fprintf(w, "Description: %s\n", d.Description)
		}
		fmt.Fprintln(w)

		if len(d.Controls) > 0 {
			fmt.Fprintln(w, "Controls:")
			for _, c := range d.Controls {
				if c.FieldName != "" {
					fmt.Fprintf(w, "  [%d] %q %s on %s\n", c.Order, c.Title, c.Type, c.FieldName)
				} else {
					fmt.Fprintf(w, "  %s\n", c.Title)
				}
			}
			fmt.Fprintln(w)
		}

		if d.GlobalQuery != "" {
			fmt.Fprintf(w, "Global query: %s\n", d.GlobalQuery)
		} else {
			fmt.Fprintln(w, "Global query: (none)")
		}
		if len(d.GlobalFilters) > 0 {
			fmt.Fprintln(w, "Global filters:")
			for _, f := range d.GlobalFilters {
				fmt.Fprintf(w, "  %s\n", f)
			}
		} else {
			fmt.Fprintln(w, "Global filters: (none)")
		}
		fmt.Fprintln(w)

		fmt.Fprintln(w, "Panels (48-column grid, sorted by position):")
		fmt.Fprintln(w)
		for _, p := range d.Panels {
			describePanel(w, p)
		}
	}
}

func describePanel(w io.Writer, p PanelInfo) {
	title := p.Title
	if title == "" {
		title = "(untitled)"
	}
	fmt.Fprintf(w, "  [%d,%d %dx%d] %q (%s)\n",
		p.GridData.X, p.GridData.Y, p.GridData.W, p.GridData.H,
		title, panelTypeString(p))

	for _, link := range p.Links {
		fmt.Fprintf(w, "    Link: %s (%s)\n", link.Label, link.Type)
	}

	for i, layer := range p.Layers {
		var cols []string
		for _, c := range layer.Columns {
			cols = append(cols, fmt.Sprintf("%s (%s)", c.SourceField, c.OperationType))
		}
		if len(p.Layers) > 1 {
			fmt.Fprintf(w, "    Layer %d: %s\n", i+1, strings.Join(cols, ", "))
		} else {
			fmt.Fprintf(w, "    Fields: %s\n", strings.Join(cols, ", "))
		}
	}

	for _, f := range p.Filters {
		fmt.Fprintf(w, "    Filter: %s\n", f)
	}

	if p.MarkdownSnippet != "" {
		fmt.Fprintf(w, "    Content: %s\n", p.MarkdownSnippet)
	}

	for _, warn := range p.Warnings {
		fmt.Fprintf(w, "    [!] %s\n", warn)
	}

	fmt.Fprintln(w)
}
