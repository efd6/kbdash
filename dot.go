package main

import (
	"fmt"
	"io"
	"strings"
)

const (
	gridScale = 0.25 // inches per grid unit (used for width/height)
	ppi       = 72.0 // points per inch — pos is in points, width/height in inches
	boxFrac   = 0.92 // fraction of grid cell used for box dimensions
	dashGap   = 4.0  // grid-unit gap between stacked dashboards
	maxFields = 8    // max field lines shown in a node label
)

func writeDOT(w io.Writer, dashboards []DashboardInfo) {
	fmt.Fprintln(w, "digraph dashboards {")
	fmt.Fprintln(w, "  splines=true;")
	fmt.Fprintln(w, `  node [shape=box, style="filled,rounded", penwidth=0.5, fillcolor=white, fontsize=11, fontname="Helvetica", fixedsize=true];`)
	fmt.Fprintln(w, `  edge [fontsize=10, fontname="Helvetica"];`)
	fmt.Fprintln(w)

	clusterOf := make(map[string]string)
	for _, d := range dashboards {
		clusterOf[d.ID] = sanitizeID(d.ID)
	}

	type dashLayout struct {
		yOffset float64
		maxY    float64
	}
	layouts := make(map[string]dashLayout)
	var cumY float64
	for _, d := range dashboards {
		var maxY float64
		for _, p := range d.Panels {
			if bottom := float64(p.GridData.Y + p.GridData.H); bottom > maxY {
				maxY = bottom
			}
		}
		layouts[d.ID] = dashLayout{yOffset: cumY, maxY: maxY}
		cumY += maxY + dashGap
	}

	firstNode := make(map[string]string)
	for _, d := range dashboards {
		if len(d.Panels) > 0 {
			firstNode[d.ID] = nodeID(clusterOf[d.ID], d.Panels[0].PanelIndex)
		}
	}

	// Emit frame nodes first so they draw behind panel nodes.
	const framePad = 1.5 // grid units of padding around panels
	for _, d := range dashboards {
		lo := layouts[d.ID]
		if len(d.Panels) == 0 {
			continue
		}
		var gMinX, gMaxX, gMinY, gMaxY float64
		for i, p := range d.Panels {
			left := float64(p.GridData.X)
			right := float64(p.GridData.X + p.GridData.W)
			top := float64(p.GridData.Y) + lo.yOffset
			bottom := float64(p.GridData.Y+p.GridData.H) + lo.yOffset
			if i == 0 {
				gMinX, gMaxX, gMinY, gMaxY = left, right, top, bottom
			} else {
				if left < gMinX {
					gMinX = left
				}
				if right > gMaxX {
					gMaxX = right
				}
				if top < gMinY {
					gMinY = top
				}
				if bottom > gMaxY {
					gMaxY = bottom
				}
			}
		}
		cx := ((gMinX + gMaxX) / 2) * gridScale * ppi
		cy := -((gMinY + gMaxY) / 2) * gridScale * ppi
		fw := (gMaxX - gMinX + 2*framePad) * gridScale
		fh := (gMaxY - gMinY + 2*framePad) * gridScale
		frameID := "frame_" + sanitizeID(d.ID)
		label := dotEscape(d.Title) + `\l`
		fmt.Fprintf(w, "  %s [label=%s, shape=box, style=%s, fillcolor=%s, color=%s, penwidth=2, fontsize=13, fontname=%s, labelloc=t, fixedsize=true, pos=\"%.2f,%.2f!\", width=%.2f, height=%.2f];\n",
			frameID, dotQuote(label),
			dotQuote("rounded,filled"), dotQuote("#f5f5f5"), dotQuote("#555555"),
			dotQuote("Helvetica Bold"),
			cx, cy, fw, fh)
	}
	fmt.Fprintln(w)

	for _, d := range dashboards {
		cn := clusterOf[d.ID]
		lo := layouts[d.ID]

		for _, p := range d.Panels {
			nid := nodeID(cn, p.PanelIndex)
			posX := (float64(p.GridData.X) + float64(p.GridData.W)/2) * gridScale * ppi
			posY := -((float64(p.GridData.Y)+float64(p.GridData.H)/2)+lo.yOffset) * gridScale * ppi
			width := float64(p.GridData.W) * gridScale * boxFrac
			height := float64(p.GridData.H) * gridScale * boxFrac

			label := dotNodeLabel(p)
			fmt.Fprintf(w, "  %s [label=%s, pos=\"%.2f,%.2f!\", width=%.2f, height=%.2f, fillcolor=%s];\n",
				nid, dotQuote(label), posX, posY, width, height, panelFill(p))
		}
		fmt.Fprintln(w)
	}

	placeholders := make(map[string]bool)
	var placeholderIdx int
	for _, d := range dashboards {
		cn := clusterOf[d.ID]
		lo := layouts[d.ID]
		for _, p := range d.Panels {
			if p.Type != "links" {
				continue
			}
			src := nodeID(cn, p.PanelIndex)
			for _, link := range p.Links {
				if link.Type != "dashboardLink" || link.DestID == "" {
					continue
				}
				if target, ok := firstNode[link.DestID]; ok {
					fmt.Fprintf(w, "  %s -> %s [label=%s];\n", src, target, dotQuote(dotEscape(link.Label)))
				} else {
					placeholder := sanitizeID(link.DestID) + "_ref"
					if !placeholders[placeholder] {
						pX := (6.0 + float64(placeholderIdx)*10.0) * gridScale * ppi
						pY := -(lo.yOffset + lo.maxY + 3) * gridScale * ppi
						fmt.Fprintf(w, "  %s [label=%s, shape=ellipse, style=dashed, fixedsize=false, fontsize=10, pos=\"%.2f,%.2f!\"];\n",
							placeholder, dotQuote(dotEscape(link.Label)), pX, pY)
						placeholders[placeholder] = true
						placeholderIdx++
					}
					fmt.Fprintf(w, "  %s -> %s;\n", src, placeholder)
				}
			}
		}
	}

	fmt.Fprintln(w, "}")
}

func panelFill(p PanelInfo) string {
	switch p.Type {
	case "links":
		return `"#ffffcc"`
	case "visualization", "legacy_vis":
		return `"#e8f5e9"`
	case "search", "discover_session":
		return `"#e3f2fd"`
	case "map":
		return `"#fff3e0"`
	default:
		return `white`
	}
}

func dotNodeLabel(p PanelInfo) string {
	var lines []string

	title := p.Title
	if title == "" {
		title = "(untitled)"
	}
	if p.HiddenTitle {
		title += " [hidden]"
	}
	lines = append(lines, title)
	lines = append(lines, panelTypeString(p))

	var fieldCount int
	for _, layer := range p.Layers {
		if layer.IgnoreGlobalFilters {
			lines = append(lines, "[ignores global filters]")
		}
		for _, c := range layer.Columns {
			if c.SourceField == "___records___" || c.SourceField == "Records" {
				continue
			}
			fieldCount++
			if fieldCount <= maxFields {
				field := c.SourceField
				if len(c.SecondaryFields) > 0 {
					field += "+" + strings.Join(c.SecondaryFields, "+")
				}
				if c.Formula != "" {
					field = truncate(c.Formula, 40)
				}
				if c.OperationType != "" {
					lines = append(lines, field+" ("+c.OperationType+")")
				} else {
					lines = append(lines, field)
				}
			}
		}
	}
	if fieldCount > maxFields {
		lines = append(lines, fmt.Sprintf("... and %d more", fieldCount-maxFields))
	}

	if len(p.Links) > 0 {
		if len(p.Links) <= 3 {
			var labels []string
			for _, l := range p.Links {
				labels = append(labels, l.Label)
			}
			lines = append(lines, strings.Join(labels, ", "))
		} else {
			lines = append(lines, fmt.Sprintf("%d dashboard links", len(p.Links)))
		}
	}

	for i, line := range lines {
		lines[i] = dotEscape(line)
	}
	return strings.Join(lines, `\l`) + `\l`
}

// nodeID produces a stable DOT identifier from the cluster name
// and the panel's UUID. First 8 hex chars of the UUID are enough
// to avoid collisions within a package.
func nodeID(cluster, panelIndex string) string {
	id := strings.ReplaceAll(panelIndex, "-", "")
	if len(id) > 8 {
		id = id[:8]
	}
	cl := cluster
	if len(cl) > 12 {
		cl = cl[:12]
	}
	return "p_" + cl + "_" + id
}

func sanitizeID(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
			b.WriteRune(c)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// dotEscape escapes characters that are special in DOT strings
// (backslash and double quote) in plain text content, before
// DOT formatting sequences like \l are added.
func dotEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// dotQuote wraps s in double quotes for DOT. The string must
// already contain any DOT escape sequences (\l, \n) — this
// function does not escape backslashes, so those sequences
// pass through to Graphviz.
func dotQuote(s string) string {
	return `"` + s + `"`
}
