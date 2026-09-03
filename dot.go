package main

import (
	"fmt"
	"io"
	"strings"
)

const (
	gridScale         = 0.25 // inches per grid unit (used for width/height)
	ppi               = 72.0 // points per inch — pos is in points, width/height in inches
	boxFrac           = 0.92 // fraction of grid cell used for box dimensions
	dashGap           = 4.0  // grid-unit gap between stacked dashboards
	sectionGap        = 2.0  // grid-unit gap between sections within a dashboard
	sectionHeaderH    = 1.5  // grid units reserved for the section label row
	maxFields         = 8    // max field lines shown in a node label
	charsPerGridWidth = 2.5  // rough characters per grid-unit of panel width (11pt Helvetica)
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
		yOffset    float64
		maxY       float64
		secOffsets map[string]float64 // section title → absolute Y offset
		secOrder   []string           // section titles in panel order
		secMaxH    map[string]float64 // section title → max (Y+H) within section
	}
	layouts := make(map[string]dashLayout)
	var cumY float64
	for _, d := range dashboards {
		var secOrder []string
		secMaxH := make(map[string]float64)
		secSeen := make(map[string]bool)
		for _, p := range d.Panels {
			bottom := float64(p.GridData.Y + p.GridData.H)
			if bottom > secMaxH[p.SectionTitle] {
				secMaxH[p.SectionTitle] = bottom
			}
			if !secSeen[p.SectionTitle] {
				secSeen[p.SectionTitle] = true
				secOrder = append(secOrder, p.SectionTitle)
			}
		}
		secOffsets := make(map[string]float64, len(secOrder))
		var cumSec float64
		for _, sec := range secOrder {
			secOffsets[sec] = cumSec
			h := secMaxH[sec]
			if sec != "" {
				h += sectionHeaderH // room for the section label row
			}
			cumSec += h + sectionGap
		}
		var maxY float64
		for _, sec := range secOrder {
			h := secMaxH[sec]
			if sec != "" {
				h += sectionHeaderH
			}
			if abs := secOffsets[sec] + h; abs > maxY {
				maxY = abs
			}
		}
		layouts[d.ID] = dashLayout{
			yOffset:    cumY,
			maxY:       maxY,
			secOffsets: secOffsets,
			secOrder:   secOrder,
			secMaxH:    secMaxH,
		}
		cumY += maxY + dashGap
	}

	firstNode := make(map[string]string)
	for _, d := range dashboards {
		if len(d.Panels) > 0 {
			firstNode[d.ID] = nodeID(clusterOf[d.ID], d.Panels[0].PanelIndex)
		}
	}

	// Emit frame and section nodes first so they draw behind panel nodes.
	const framePad = 1.5 // grid units of padding around panels
	for _, d := range dashboards {
		lo := layouts[d.ID]
		if len(d.Panels) == 0 {
			continue
		}
		var gMinX, gMaxX, gMinY, gMaxY float64
		for i, p := range d.Panels {
			absY := panelAbsY(p, lo.secOffsets)
			left := float64(p.GridData.X)
			right := float64(p.GridData.X + p.GridData.W)
			top := absY + lo.yOffset
			bottom := absY + float64(p.GridData.H) + lo.yOffset
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
		// Add extra top padding so the dashboard title label
		// clears the top edge of the first section box.
		frameTopY := gMinY - framePad - sectionHeaderH
		frameBottomY := gMaxY + framePad
		cx := ((gMinX + gMaxX) / 2) * gridScale * ppi
		cy := -((frameTopY + frameBottomY) / 2) * gridScale * ppi
		fw := (gMaxX - gMinX + 2*framePad) * gridScale
		fh := (frameBottomY - frameTopY) * gridScale
		frameID := "frame_" + sanitizeID(d.ID)
		label := dotEscape(d.Title) + `\l`
		fmt.Fprintf(w, "  %s [label=%s, shape=box, style=%s, fillcolor=%s, color=%s, penwidth=2, fontsize=13, fontname=%s, labelloc=t, fixedsize=true, pos=\"%.2f,%.2f!\", width=%.2f, height=%.2f];\n",
			frameID, dotQuote(label),
			dotQuote("rounded,filled"), dotQuote("#f5f5f5"), dotQuote("#555555"),
			dotQuote("Helvetica Bold"),
			cx, cy, fw, fh)

		for _, sec := range lo.secOrder {
			if sec == "" {
				continue // unsectioned dashboards need no section frame
			}
			secTop := lo.secOffsets[sec] + lo.yOffset
			secBot := lo.secOffsets[sec] + sectionHeaderH + lo.secMaxH[sec] + lo.yOffset
			scx := 24.0 * gridScale * ppi
			scy := -((secTop + secBot) / 2) * gridScale * ppi
			sfw := 48.0 * gridScale
			sfh := (secBot - secTop) * gridScale
			secID := "sec_" + sanitizeID(d.ID) + "_" + sanitizeID(sec)
			slabel := dotEscape(sec) + `\l`
			fmt.Fprintf(w, "  %s [label=%s, shape=box, style=%s, fillcolor=%s, color=%s, penwidth=1, fontsize=11, fontname=%s, labelloc=t, fixedsize=true, pos=\"%.2f,%.2f!\", width=%.2f, height=%.2f];\n",
				secID, dotQuote(slabel),
				dotQuote("rounded,filled"), dotQuote("#f0f4ff"), dotQuote("#9999cc"),
				dotQuote("Helvetica"),
				scx, scy, sfw, sfh)
		}
	}
	fmt.Fprintln(w)

	for _, d := range dashboards {
		cn := clusterOf[d.ID]
		lo := layouts[d.ID]

		for _, p := range d.Panels {
			nid := nodeID(cn, p.PanelIndex)
			absY := panelAbsY(p, lo.secOffsets)
			posX := (float64(p.GridData.X) + float64(p.GridData.W)/2) * gridScale * ppi
			posY := -((absY + float64(p.GridData.H)/2) + lo.yOffset) * gridScale * ppi
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

// panelAbsY returns the absolute Y position of panel p, accounting for
// per-section offsets and the header row reserved for the section label.
func panelAbsY(p PanelInfo, secOffsets map[string]float64) float64 {
	y := float64(p.GridData.Y) + secOffsets[p.SectionTitle]
	if p.SectionTitle != "" {
		y += sectionHeaderH
	}
	return y
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
	maxChars := int(float64(p.GridData.W)*charsPerGridWidth + 0.5)
	if maxChars < 10 {
		maxChars = 10
	}
	for _, wrapped := range wordWrap(title, maxChars) {
		lines = append(lines, wrapped)
	}
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
				var line string
				if c.OperationType != "" {
					line = field + " (" + c.OperationType + ")"
				} else {
					line = field
				}
				lines = append(lines, truncate(line, maxChars))
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

// wordWrap splits text into lines of at most maxChars characters,
// breaking at word boundaries where possible.
func wordWrap(text string, maxChars int) []string {
	if len(text) <= maxChars {
		return []string{text}
	}
	var lines []string
	words := strings.Fields(text)
	var cur strings.Builder
	for _, w := range words {
		if cur.Len() == 0 {
			cur.WriteString(w)
			continue
		}
		if cur.Len()+1+len(w) <= maxChars {
			cur.WriteByte(' ')
			cur.WriteString(w)
		} else {
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(w)
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}
