package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// Raw JSON types matching Kibana saved-object exports.

type Dashboard struct {
	Attributes Attributes  `json:"attributes"`
	ID         string      `json:"id"`
	References []Reference `json:"references"`
}

type Reference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type Attributes struct {
	Title             string        `json:"title"`
	Description       string        `json:"description"`
	PanelsJSON        []Panel       `json:"panelsJSON"`
	ControlGroupInput *ControlGroup `json:"controlGroupInput,omitempty"`
	KibanaMeta        KibanaMeta    `json:"kibanaSavedObjectMeta"`
}

type KibanaMeta struct {
	SearchSourceRaw json.RawMessage `json:"searchSourceJSON"`
}

type SearchSource struct {
	Filter []json.RawMessage `json:"filter"`
	Query  Query             `json:"query"`
}

type Query struct {
	Language string `json:"language"`
	Query    string `json:"query"`
}

type ControlGroup struct {
	PanelsRaw json.RawMessage `json:"panelsJSON"`
}

type RawControl struct {
	Type          string          `json:"type"`
	Order         int             `json:"order"`
	ExplicitInput json.RawMessage `json:"explicitInput"`
}

type ControlInput struct {
	FieldName  string `json:"fieldName"`
	Title      string `json:"title"`
	DataViewID string `json:"dataViewId"`
}

type Panel struct {
	Type             string          `json:"type"`
	Title            string          `json:"title"`
	PanelIndex       string          `json:"panelIndex"`
	PanelRefName     string          `json:"panelRefName,omitempty"`
	GridData         GridData        `json:"gridData"`
	EmbeddableConfig json.RawMessage `json:"embeddableConfig"`
}

type GridData struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Lens panel types.

type LensEmbeddable struct {
	Attributes struct {
		Title             string          `json:"title"`
		VisualizationType string          `json:"visualizationType"`
		State             json.RawMessage `json:"state"`
	} `json:"attributes"`
}

type FlexQuery struct {
	Language string `json:"language"`
	Query    string `json:"query"`
	ESQL     string `json:"esql"`
}

type LensState struct {
	DatasourceStates struct {
		FormBased    *FormBasedDS `json:"formBased,omitempty"`
		IndexPattern *FormBasedDS `json:"indexpattern,omitempty"`
		TextBased    *TextBasedDS `json:"textBased,omitempty"`
	} `json:"datasourceStates"`
	Filters       json.RawMessage `json:"filters,omitempty"`
	Query         *FlexQuery      `json:"query,omitempty"`
	Visualization json.RawMessage `json:"visualization,omitempty"`
}

type FormBasedDS struct {
	Layers map[string]FormBasedLayer `json:"layers"`
}

type TextBasedDS struct {
	Layers map[string]json.RawMessage `json:"layers"`
}

type FormBasedLayer struct {
	ColumnOrder         []string          `json:"columnOrder"`
	Columns             map[string]Column `json:"columns"`
	IgnoreGlobalFilters bool              `json:"ignoreGlobalFilters,omitempty"`
}

type Column struct {
	Label         string       `json:"label"`
	SourceField   string       `json:"sourceField"`
	OperationType string       `json:"operationType"`
	DataType      string       `json:"dataType"`
	CustomLabel   bool         `json:"customLabel"`
	Filter        *FlexQuery   `json:"filter,omitempty"`
	Params        ColumnParams `json:"params"`
}

type ColumnParams struct {
	Formula         string   `json:"formula,omitempty"`
	SecondaryFields []string `json:"secondaryFields,omitempty"`
}

type XYVisualization struct {
	Layers              []XYLayer `json:"layers"`
	PreferredSeriesType string    `json:"preferredSeriesType"`
}

type XYLayer struct {
	SeriesType string `json:"seriesType"`
}

// Links panel types.

type LinksEmbeddable struct {
	Attributes struct {
		Links []LinkEntry `json:"links"`
	} `json:"attributes"`
}

type LinkEntry struct {
	Label              string `json:"label"`
	Type               string `json:"type"`
	DestinationRefName string `json:"destinationRefName"`
	Order              int    `json:"order"`
}

// Legacy visualization panel types.

type VisualizationEmbeddable struct {
	SavedVis struct {
		Type   string `json:"type"`
		Params struct {
			Markdown string `json:"markdown"`
		} `json:"params"`
	} `json:"savedVis"`
}

// Extracted/normalised info.

type DashboardInfo struct {
	Title         string
	Description   string
	File          string
	ID            string
	Controls      []ControlInfo
	GlobalQuery   string
	GlobalFilters []string
	Panels        []PanelInfo
	References    []Reference
}

type ControlInfo struct {
	Order     int
	Title     string
	Type      string
	FieldName string
}

type PanelInfo struct {
	Title           string
	HiddenTitle     bool
	Type            string
	SubType         string
	SeriesType      string
	GridData        GridData
	PanelIndex      string
	Layers          []LayerInfo
	Links           []LinkInfo
	Filters         []string
	Warnings        []string
	RefID           string
	MarkdownSnippet string
}

type LayerInfo struct {
	Columns             []ColumnInfo
	IgnoreGlobalFilters bool
}

type ColumnInfo struct {
	Label           string
	SourceField     string
	SecondaryFields []string
	OperationType   string
	Formula         string
	Filter          string
}

type LinkInfo struct {
	Label  string
	Type   string
	DestID string
}

// Loading and file discovery.

func loadDashboard(path string) (*Dashboard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d Dashboard
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &d, nil
}

func findDashboardFiles(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if strings.HasSuffix(path, ".json") {
				files = append(files, path)
			}
			continue
		}
		dashDir := filepath.Join(path, "kibana", "dashboard")
		if fi, err := os.Stat(dashDir); err == nil && fi.IsDir() {
			path = dashDir
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				files = append(files, filepath.Join(path, e.Name()))
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

// decodeStringOrObject tries to unmarshal raw as T; if that fails,
// tries to unmarshal as a JSON string and then decode that string as T.
func decodeStringOrObject[T any](raw json.RawMessage) (T, error) {
	var result T
	if len(raw) == 0 {
		return result, nil
	}
	if err := json.Unmarshal(raw, &result); err == nil {
		return result, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		var zero T
		return zero, fmt.Errorf("not an object or string: %s", truncate(string(raw), 80))
	}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		var zero T
		return zero, fmt.Errorf("decoding string content: %w", err)
	}
	return result, nil
}

// Extraction.

func extractDashboard(d *Dashboard, file string) DashboardInfo {
	info := DashboardInfo{
		Title:       d.Attributes.Title,
		Description: d.Attributes.Description,
		File:        filepath.Base(file),
		ID:          d.ID,
		References:  d.References,
	}

	if d.Attributes.ControlGroupInput != nil {
		controls, err := parseControls(d.Attributes.ControlGroupInput.PanelsRaw)
		if err != nil {
			info.Controls = append(info.Controls, ControlInfo{
				Title: fmt.Sprintf("(parse error: %v)", err),
			})
		} else {
			info.Controls = controls
		}
	}

	if len(d.Attributes.KibanaMeta.SearchSourceRaw) > 0 {
		ss, err := decodeStringOrObject[SearchSource](d.Attributes.KibanaMeta.SearchSourceRaw)
		if err == nil {
			info.GlobalQuery = ss.Query.Query
			for _, f := range ss.Filter {
				if desc := describeRawFilter(f); desc != "" {
					info.GlobalFilters = append(info.GlobalFilters, desc)
				}
			}
		}
	}

	for _, p := range d.Attributes.PanelsJSON {
		info.Panels = append(info.Panels, extractPanel(p, d.References))
	}
	sort.Slice(info.Panels, func(i, j int) bool {
		if info.Panels[i].GridData.Y != info.Panels[j].GridData.Y {
			return info.Panels[i].GridData.Y < info.Panels[j].GridData.Y
		}
		return info.Panels[i].GridData.X < info.Panels[j].GridData.X
	})

	return info
}

func parseControls(raw json.RawMessage) ([]ControlInfo, error) {
	rawMap, err := decodeStringOrObject[map[string]RawControl](raw)
	if err != nil {
		return nil, err
	}
	var controls []ControlInfo
	for _, rc := range rawMap {
		var input ControlInput
		if len(rc.ExplicitInput) > 0 {
			_ = json.Unmarshal(rc.ExplicitInput, &input)
		}
		controls = append(controls, ControlInfo{
			Order:     rc.Order,
			Title:     input.Title,
			Type:      rc.Type,
			FieldName: input.FieldName,
		})
	}
	sort.Slice(controls, func(i, j int) bool {
		return controls[i].Order < controls[j].Order
	})
	return controls, nil
}

func extractPanel(p Panel, refs []Reference) PanelInfo {
	pi := PanelInfo{
		Title:      p.Title,
		Type:       p.Type,
		GridData:   p.GridData,
		PanelIndex: p.PanelIndex,
	}

	// Newer Kibana exports place the panel display title and the
	// hidden-title flag inside embeddableConfig rather than at the
	// top level.
	var ec struct {
		Title           string `json:"title"`
		HidePanelTitles *bool  `json:"hidePanelTitles,omitempty"`
	}
	if json.Unmarshal(p.EmbeddableConfig, &ec) == nil {
		if ec.Title != "" {
			if pi.Title == "" {
				pi.Title = ec.Title
			} else if pi.Title != ec.Title {
				pi.Warnings = append(pi.Warnings, fmt.Sprintf("title mismatch: panel %q vs embeddableConfig %q", pi.Title, ec.Title))
			}
		}
		if ec.HidePanelTitles != nil && *ec.HidePanelTitles {
			pi.HiddenTitle = true
		}
	}

	switch p.Type {
	case "lens", "vis":
		extractLens(&pi, p)
	case "links":
		extractLinks(&pi, p.EmbeddableConfig, p.PanelIndex, refs)
	case "visualization", "legacy_vis":
		extractVisualization(&pi, p.EmbeddableConfig)
	case "search", "discover_session":
		extractSearch(&pi, p, refs)
	case "map":
		extractMap(&pi, p.EmbeddableConfig)
	default:
		pi.Warnings = append(pi.Warnings, fmt.Sprintf("unknown panel type: %s", p.Type))
	}

	extractEmbeddableFilters(&pi, p.EmbeddableConfig)
	return pi
}

func extractLens(pi *PanelInfo, p Panel) {
	var le LensEmbeddable
	if err := json.Unmarshal(p.EmbeddableConfig, &le); err != nil {
		pi.Warnings = append(pi.Warnings, fmt.Sprintf("lens parse error: %v", err))
		return
	}
	pi.SubType = le.Attributes.VisualizationType

	// Title consistency: effective panel title vs lens attribute title.
	if le.Attributes.Title != "" {
		if pi.Title == "" {
			pi.Title = le.Attributes.Title
		} else if pi.Title != le.Attributes.Title {
			pi.Warnings = append(pi.Warnings, fmt.Sprintf("title mismatch: panel %q vs lens %q", pi.Title, le.Attributes.Title))
		}
	}

	// Extract panel-level query (embeddableConfig.query).
	var ec struct {
		Query *FlexQuery `json:"query"`
	}
	if err := json.Unmarshal(p.EmbeddableConfig, &ec); err != nil {
		pi.Warnings = append(pi.Warnings, fmt.Sprintf("embeddableConfig query parse error: %v", err))
	}

	if len(le.Attributes.State) == 0 {
		return
	}

	var state LensState
	if err := json.Unmarshal(le.Attributes.State, &state); err != nil {
		pi.Warnings = append(pi.Warnings, fmt.Sprintf("lens state parse error: %v", err))
		return
	}

	var ds *FormBasedDS
	if state.DatasourceStates.FormBased != nil && len(state.DatasourceStates.FormBased.Layers) > 0 {
		ds = state.DatasourceStates.FormBased
	} else if state.DatasourceStates.IndexPattern != nil && len(state.DatasourceStates.IndexPattern.Layers) > 0 {
		ds = state.DatasourceStates.IndexPattern
	}
	if ds != nil {
		for _, layer := range ds.Layers {
			li := LayerInfo{
				IgnoreGlobalFilters: layer.IgnoreGlobalFilters,
			}
			for _, colID := range layer.ColumnOrder {
				col, ok := layer.Columns[colID]
				if !ok {
					continue
				}
				ci := ColumnInfo{
					Label:           col.Label,
					SourceField:     col.SourceField,
					SecondaryFields: col.Params.SecondaryFields,
					OperationType:   col.OperationType,
					Formula:         col.Params.Formula,
				}
				if col.Filter != nil {
					q := col.Filter.Query
					if q == "" {
						q = col.Filter.ESQL
					}
					ci.Filter = q
				}
				li.Columns = append(li.Columns, ci)
			}
			if len(li.Columns) > 0 {
				pi.Layers = append(pi.Layers, li)
			}
		}
	}

	// ES|QL consistency: compare datasource, state, and panel copies.
	if state.DatasourceStates.TextBased != nil {
		hasESQL := false
		queries := make(map[string][]string) // query text -> locations
		for _, raw := range state.DatasourceStates.TextBased.Layers {
			var layer struct {
				Query struct {
					ESQL string `json:"esql"`
				} `json:"query"`
			}
			if json.Unmarshal(raw, &layer) == nil && layer.Query.ESQL != "" {
				hasESQL = true
				queries[layer.Query.ESQL] = append(queries[layer.Query.ESQL], "datasource")
			}
		}
		if state.Query != nil && state.Query.ESQL != "" {
			hasESQL = true
			queries[state.Query.ESQL] = append(queries[state.Query.ESQL], "state")
		}
		if ec.Query != nil && ec.Query.ESQL != "" {
			hasESQL = true
			queries[ec.Query.ESQL] = append(queries[ec.Query.ESQL], "panel")
		}
		if len(queries) > 1 {
			pi.Warnings = append(pi.Warnings, "ES|QL query mismatch: datasource, state, and panel copies differ")
		}
		if hasESQL && len(pi.Layers) == 0 {
			pi.Warnings = append(pi.Warnings, "ES|QL datasource (fields not extracted)")
		}
	}

	// KQL consistency: state-level query vs panel-level query.
	// Only flag when both are non-empty and different. An empty
	// panel-level query is normal — it means the dashboard doesn't
	// override the Lens visualization's built-in query.
	stateKQL := ""
	if state.Query != nil && state.Query.Language == "kuery" {
		stateKQL = state.Query.Query
	}
	panelKQL := ""
	if ec.Query != nil && ec.Query.Language == "kuery" {
		panelKQL = ec.Query.Query
	}
	if stateKQL != "" && panelKQL != "" && stateKQL != panelKQL {
		pi.Warnings = append(pi.Warnings, fmt.Sprintf("KQL query mismatch: state %q vs panel %q", stateKQL, panelKQL))
	}

	if pi.SubType == "lnsXY" && len(state.Visualization) > 0 {
		var xyViz XYVisualization
		if err := json.Unmarshal(state.Visualization, &xyViz); err == nil {
			if len(xyViz.Layers) > 0 && xyViz.Layers[0].SeriesType != "" {
				pi.SeriesType = xyViz.Layers[0].SeriesType
			} else if xyViz.PreferredSeriesType != "" {
				pi.SeriesType = xyViz.PreferredSeriesType
			}
		}
	}

	if len(state.Filters) > 0 {
		var filters []json.RawMessage
		if err := json.Unmarshal(state.Filters, &filters); err == nil {
			for _, f := range filters {
				if desc := describeRawFilter(f); desc != "" {
					pi.Filters = append(pi.Filters, desc)
				}
			}
		}
	}
}

func extractLinks(pi *PanelInfo, raw json.RawMessage, panelIndex string, refs []Reference) {
	var le LinksEmbeddable
	if err := json.Unmarshal(raw, &le); err != nil {
		pi.Warnings = append(pi.Warnings, fmt.Sprintf("links parse error: %v", err))
		return
	}

	refMap := make(map[string]Reference)
	for _, r := range refs {
		refMap[r.Name] = r
	}

	sort.Slice(le.Attributes.Links, func(i, j int) bool {
		return le.Attributes.Links[i].Order < le.Attributes.Links[j].Order
	})

	for _, link := range le.Attributes.Links {
		li := LinkInfo{
			Label: link.Label,
			Type:  link.Type,
		}
		refKey := panelIndex + ":" + link.DestinationRefName
		if ref, ok := refMap[refKey]; ok {
			li.DestID = ref.ID
		}
		pi.Links = append(pi.Links, li)
	}
}

func extractVisualization(pi *PanelInfo, raw json.RawMessage) {
	var ve VisualizationEmbeddable
	if err := json.Unmarshal(raw, &ve); err != nil {
		pi.Warnings = append(pi.Warnings, fmt.Sprintf("visualization parse error: %v", err))
		return
	}
	pi.SubType = ve.SavedVis.Type
	if ve.SavedVis.Type == "markdown" && ve.SavedVis.Params.Markdown != "" {
		pi.MarkdownSnippet = truncate(ve.SavedVis.Params.Markdown, 120)
	}
}

func extractSearch(pi *PanelInfo, p Panel, refs []Reference) {
	// Try by-value first: the search definition is inlined in
	// embeddableConfig.attributes.
	if extractInlineSearch(pi, p.EmbeddableConfig) {
		return
	}

	// Fall back to by-reference resolution.
	for _, r := range refs {
		if r.Name == p.PanelIndex+":"+p.PanelRefName {
			pi.RefID = r.ID
			break
		}
	}
	if pi.RefID == "" {
		for _, r := range refs {
			if strings.HasSuffix(r.Name, p.PanelRefName) && r.Type == "search" {
				pi.RefID = r.ID
				break
			}
		}
	}
	// Newer exports store the saved object ID directly in
	// embeddableConfig.savedObjectId.
	if pi.RefID == "" {
		var ec struct {
			SavedObjectID string `json:"savedObjectId"`
		}
		if json.Unmarshal(p.EmbeddableConfig, &ec) == nil && ec.SavedObjectID != "" {
			pi.RefID = ec.SavedObjectID
		}
	}
	if pi.RefID != "" {
		pi.Warnings = append(pi.Warnings, fmt.Sprintf("saved search ref: %s (definition not inlined)", pi.RefID))
	} else {
		pi.Warnings = append(pi.Warnings, "saved search reference could not be resolved")
	}
}

// extractInlineSearch handles by-value search panels where the full
// definition is embedded in embeddableConfig.attributes.
func extractInlineSearch(pi *PanelInfo, raw json.RawMessage) bool {
	var ec struct {
		Attributes struct {
			Columns    []string   `json:"columns"`
			KibanaMeta KibanaMeta `json:"kibanaSavedObjectMeta"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(raw, &ec); err != nil || len(ec.Attributes.Columns) == 0 {
		return false
	}

	pi.Layers = append(pi.Layers, LayerInfo{
		Columns: searchColumns(ec.Attributes.Columns),
	})

	if len(ec.Attributes.KibanaMeta.SearchSourceRaw) > 0 {
		ss, err := decodeStringOrObject[SearchSource](ec.Attributes.KibanaMeta.SearchSourceRaw)
		if err == nil {
			for _, f := range ss.Filter {
				if desc := describeRawFilter(f); desc != "" {
					pi.Filters = append(pi.Filters, desc)
				}
			}
		}
	}
	return true
}

func searchColumns(cols []string) []ColumnInfo {
	out := make([]ColumnInfo, len(cols))
	for i, c := range cols {
		out[i] = ColumnInfo{SourceField: c}
	}
	return out
}

func extractMap(pi *PanelInfo, raw json.RawMessage) {
	var m struct {
		Attributes struct {
			LayerListJSON json.RawMessage `json:"layerListJSON"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		pi.Warnings = append(pi.Warnings, fmt.Sprintf("map parse error: %v", err))
		return
	}

	if len(m.Attributes.LayerListJSON) > 0 {
		var layers []json.RawMessage
		if err := json.Unmarshal(m.Attributes.LayerListJSON, &layers); err != nil {
			var s string
			if err2 := json.Unmarshal(m.Attributes.LayerListJSON, &s); err2 == nil {
				_ = json.Unmarshal([]byte(s), &layers)
			}
		}
		for _, layerRaw := range layers {
			var layer struct {
				SourceDescriptor struct {
					Type     string `json:"type"`
					GeoField string `json:"geoField"`
				} `json:"sourceDescriptor"`
				Type string `json:"type"`
			}
			if err := json.Unmarshal(layerRaw, &layer); err == nil && layer.SourceDescriptor.GeoField != "" {
				pi.Layers = append(pi.Layers, LayerInfo{
					Columns: []ColumnInfo{{
						SourceField:   layer.SourceDescriptor.GeoField,
						OperationType: layer.SourceDescriptor.Type,
						Label:         layer.Type,
					}},
				})
			}
		}
	}
	if len(pi.Layers) == 0 {
		pi.Warnings = append(pi.Warnings, "map layers not fully parsed")
	}
}

// extractEmbeddableFilters pulls filters from embeddableConfig.filters,
// which exists on most panel types as an override.
func extractEmbeddableFilters(pi *PanelInfo, raw json.RawMessage) {
	var ec struct {
		Filters []json.RawMessage `json:"filters"`
	}
	if err := json.Unmarshal(raw, &ec); err != nil || len(ec.Filters) == 0 {
		return
	}
	for _, f := range ec.Filters {
		desc := describeRawFilter(f)
		if desc == "" {
			continue
		}
		if !slices.Contains(pi.Filters, desc) {
			pi.Filters = append(pi.Filters, desc)
		}
	}
}

func describeRawFilter(raw json.RawMessage) string {
	var f struct {
		Meta struct {
			Alias    *string         `json:"alias"`
			Field    string          `json:"field"`
			Key      string          `json:"key"`
			Negate   bool            `json:"negate"`
			Type     string          `json:"type"`
			Relation string          `json:"relation"`
			Params   json.RawMessage `json:"params"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return "(unparseable filter)"
	}
	field := f.Meta.Field
	if field == "" {
		field = f.Meta.Key
	}

	neg := ""
	if f.Meta.Negate {
		neg = "NOT "
	}

	switch f.Meta.Type {
	case "exists":
		if field == "" {
			return ""
		}
		return fmt.Sprintf("%s%s exists", neg, field)
	case "phrases":
		var values []string
		if json.Unmarshal(f.Meta.Params, &values) == nil {
			return fmt.Sprintf("%s%s IN (%s)", neg, field, strings.Join(values, ", "))
		}
		if field == "" {
			return ""
		}
		return fmt.Sprintf("%s%s phrases (details not extracted)", neg, field)
	case "combined":
		if f.Meta.Alias != nil && *f.Meta.Alias != "" {
			return fmt.Sprintf("%s%s", neg, *f.Meta.Alias)
		}
		var subFilters []json.RawMessage
		if json.Unmarshal(f.Meta.Params, &subFilters) == nil && len(subFilters) > 0 {
			relation := strings.ToUpper(f.Meta.Relation)
			if relation == "" {
				relation = "AND"
			}
			var parts []string
			for _, sf := range subFilters {
				if desc := describeRawFilter(sf); desc != "" {
					parts = append(parts, desc)
				}
			}
			if len(parts) > 0 {
				return neg + "(" + strings.Join(parts, " "+relation+" ") + ")"
			}
		}
		return neg + "combined filter"
	case "phrase":
		if field == "" {
			return ""
		}
		op := "="
		if f.Meta.Negate {
			op = "!="
		}
		// Older format: value in meta.params.query.
		var params struct {
			Query any `json:"query"`
		}
		if json.Unmarshal(f.Meta.Params, &params) == nil && params.Query != nil {
			return fmt.Sprintf("%s %s %v", field, op, params.Query)
		}
		// Newer format: value in query.match_phrase.<field>.
		var full struct {
			Query struct {
				MatchPhrase map[string]any `json:"match_phrase"`
			} `json:"query"`
		}
		if json.Unmarshal(raw, &full) == nil {
			if val, ok := full.Query.MatchPhrase[field]; ok {
				return fmt.Sprintf("%s %s %v", field, op, val)
			}
		}
		return fmt.Sprintf("%s%s exists", neg, field)
	case "custom":
		if f.Meta.Alias != nil && *f.Meta.Alias != "" {
			return fmt.Sprintf("%s%s", neg, *f.Meta.Alias)
		}
		return neg + "custom filter"
	default:
		if field == "" {
			return ""
		}
		var params struct {
			Query any `json:"query"`
		}
		_ = json.Unmarshal(f.Meta.Params, &params)
		op := "="
		if f.Meta.Negate {
			op = "!="
		}
		return fmt.Sprintf("%s %s %v", field, op, params.Query)
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func panelTypeString(p PanelInfo) string {
	switch p.Type {
	case "lens", "vis":
		s := "lens: " + p.SubType
		if p.SeriesType != "" {
			s += ", " + p.SeriesType
		}
		return s
	case "visualization", "legacy_vis":
		if p.SubType != "" {
			return "visualization: " + p.SubType
		}
		return "visualization"
	default:
		return p.Type
	}
}
