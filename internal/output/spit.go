// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/apex/log"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli/v3"
	"gopkg.in/yaml.v2"

	"github.com/tfquery/tfquery/internal/attrs"
	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/filters"
	"github.com/tfquery/tfquery/internal/jq"
)

// InterfaceToString converts supported primitive or composite values to a
// string. A custom empty value may be provided.
func InterfaceToString(value any, emptyValue ...string) string {
	if len(emptyValue) == 0 {
		emptyValue = []string{""}
	}

	if value == nil {
		return emptyValue[0]
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer:
		if rv.IsNil() {
			return emptyValue[0]
		}
	case reflect.Map, reflect.Slice:
		if rv.Len() == 0 {
			return emptyValue[0]
		}
	}

	// We note that the int and bool cases are unlikely to be reached due to JSON
	// parsing behavior.
	switch value := value.(type) {
	case string:
		if value == "" {
			return emptyValue[0]
		}
		return value
	case int:
		return strconv.Itoa(value)
	case float64:
		// Our current use cases have no need for an actual float, so we just return
		// an integer.
		return fmt.Sprintf("%.0f", value)
	case bool:
		return strconv.FormatBool(value)
	default:
		jsonBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(jsonBytes)
	}
}

// NewTag constructs a Tag from a raw struct tag value and an optional holder
// prefix used to build hierarchical attribute names.
func NewTag(h string, s string) schemaTag {
	allowed := []string{"attr"}

	tag := schemaTag{}

	parts := strings.Split(s, ",")
	if len(parts) > 0 {
		found := slices.Contains(allowed, parts[0])

		if !found {
			return tag
		}

		tag.Kind = parts[0]
	}

	if len(parts) > 1 {
		if h != "" {
			parts[1] = fmt.Sprintf("%s.%s", h, parts[1])
		}
		tag.Name = parts[1]
	}

	if len(parts) > 2 {
		tag.Encoding = parts[2]
	}

	return tag
}

// SliceDiceSpit orchestrates filtering, transforming, sorting and rendering
// of a dataset according to command flags and attribute specifications. The
// optional postProcess callback allows commands to apply custom transformations
// to the filtered dataset before rendering.
func SliceDiceSpit(raw bytes.Buffer,
	attrs attrs.AttrList,
	cmd *cli.Command,
	parent string,
	w io.Writer,
	postProcess func([]map[string]any) ([]map[string]any, error),
) {
	// Default to stdout.
	if w == nil {
		w = os.Stdout
	}

	// If raw, just dump it and go home.
	output := cmd.String("output")
	if output == "raw" {
		_, _ = w.Write(raw.Bytes())

		if cmd.String("json-into") == "" && cmd.String("yaml-into") == "" {
			return
		}

		var rawDoc any
		if err := json.Unmarshal(raw.Bytes(), &rawDoc); err != nil {
			log.Errorf("SliceDiceSpit json unmarshal for raw into output: %v", err)
			return
		}

		writeIntoFiles(cmd, rawDoc)

		return
	}

	// Flatten the state schema, if this is sq.  This is done to bring the
	// structure of the state file into alignment with the structures found in
	// other command's payloads, thus enabling a common set of logic to process
	// all.
	if resources := gjson.Parse(raw.String()).Get("resources"); resources.Exists() {
		raw = flattenState(resources, !cmd.Bool("short"))
	}

	var fullDataset gjson.Result
	// We keep the "data" object from the document and throw away everything
	// else, notably "included", which we don't have a use case for. We also
	// parse this into JSON so that we can use the lowercase key names and not
	// the proper case names from the TFE API.
	if parent != "" {
		fullDataset = gjson.Parse(raw.String()).Get(parent)
	} else {
		fullDataset = gjson.Parse(raw.String())
	}

	// Filter out the rows we don't want. Do it here so that the following
	// processes are slightly more efficient since they'll be working on a smaller
	// dataset.
	filterSpec := cmd.String("filter")
	jqSpec := cmd.String("jq")

	if filterSpec != "" && jqSpec != "" {
		msg := "flags --filter and --jq are mutually exclusive"
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		log.Error(msg)
		return
	}

	var filteredDataset []map[string]any
	if jqSpec != "" {
		var err error
		filteredDataset, err = jq.FilterDatasetJQ(fullDataset, attrs, jqSpec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: invalid jq query: %v\n", err)
			log.Errorf("SliceDiceSpit jq filter: %v", err)
			return
		}
	} else {
		// FilterDataset() will trap the filterSpec == "" case and just return the
		// full dataset transformed into a []map[string]interface{} with the
		// specified attributes included.
		filteredDataset = filters.FilterDataset(fullDataset, attrs, filterSpec)
	}

	// THINK Force a time transformation to occur for all attributes, even though
	// many will not be a timestamp. One alternative would be to look at first row
	// of full dataset and only add the time transformation to attrs that look
	// like timestamps.
	if cmd.Bool("local") {
		for a := range attrs {
			attrs[a].TransformSpec += "t"
		}
	}

	// Transform each value in each row.
	for _, row := range filteredDataset {
		for _, attr := range attrs {
			if attr.TransformSpec != "" {
				row[attr.OutputKey] = attr.Transform(row[attr.OutputKey])
			}
		}
	}

	// Apply command-specific post-processing.
	var err error
	if postProcess != nil {
		if filteredDataset, err = postProcess(filteredDataset); err != nil {
			log.Errorf("PostProcess: %v", err)
		}
	}

	// Finally, let's sort it. This is done after all the other dataset processing
	// so that we're sorting on the final values that will be displayed.
	spec := cmd.String("sort")
	SortDataset(filteredDataset, spec)

	switch output {
	case "json":
		// We marshal the filtered dataset into a JSON document. Note that the JSON
		// key order will not match the order of the fields in filteredDataset since
		// we're using maps to represent rows. The Go encoding/json package
		// intentionally sorts the keys in the output.
		jsonOutput, err := json.Marshal(filteredDataset)
		if err != nil {
			log.Errorf("SliceDiceSpit json marshal: %v", err)
		}
		os.Stdout.Write(jsonOutput)
	case "yaml":
		yamlOutput, err := yaml.Marshal(filteredDataset)
		if err != nil {
			log.Errorf("SliceDiceSpit yaml marshal: %v", err)
		}
		os.Stdout.Write(yamlOutput)
	default:
		TableWriter(filteredDataset, attrs, cmd, w)
	}

	writeIntoFiles(cmd, filteredDataset)
}

// TableWriter renders the result set in a tabular form honoring color,
// titles and padding options. Output is written to w. If w is nil, os.Stdout
// is used.
func TableWriter(resultSet []map[string]any, attrs attrs.AttrList, cmd *cli.Command, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}

	// We return early if there are no results to display.
	if len(resultSet) == 0 {
		return
	}

	// We initialize the table styles.
	var (
		headerStyle  = lipgloss.NewStyle().Align(lipgloss.Left).Bold(true)
		cellStyle    = lipgloss.NewStyle().Padding(0, 0).Align(lipgloss.Left)
		evenRowStyle = cellStyle
		oddRowStyle  = cellStyle
	)

	// And then color styles if --color is present.
	if cmd.Bool("color") {
		headerColor, evenColor, oddColor := getColors("colors")

		headerStyle = headerStyle.Foreground(headerColor)
		evenRowStyle = evenRowStyle.Foreground(evenColor)
		oddRowStyle = oddRowStyle.Foreground(oddColor)
	}

	// We build the table rows from the result set.
	var rows [][]string
	for _, result := range resultSet {
		row := make([]string, 0, len(result))
		for _, attr := range attrs {
			if !attr.Include {
				continue
			}
			row = append(row, InterfaceToString(result[attr.OutputKey], "-"))
		}
		rows = append(rows, row)
	}

	// We render the header if present.
	if cmd.Metadata["header"] != nil {
		fmt.Fprintln(w, headerStyle.Render(cmd.Metadata["header"].(string)))
	}

	// We configure the table with padding and styles.
	pad := cmd.Int("padding")
	// pad, _ := config.GetInt("padding", 0)
	t := table.New().
		BorderBottom(false).
		BorderTop(false).
		BorderLeft(false).
		BorderRight(false).
		Border(lipgloss.HiddenBorder()).
		StyleFunc(func(row, col int) lipgloss.Style {
			var style lipgloss.Style
			switch {
			case row == table.HeaderRow:
				style = headerStyle
			case row%2 == 0:
				style = evenRowStyle
			default:
				style = oddRowStyle
			}

			if col > 0 {
				style = style.PaddingLeft(pad)
			}

			return style
		}).
		Headers().
		Rows(rows...)

	// We add column headers if titles are enabled.
	if cmd.Bool("titles") {
		var headers []string
		for _, attr := range attrs {
			if attr.Include {
				headers = append(headers, attr.OutputKey)
			}
		}

		// https://github.com/charmbracelet/lipgloss/issues/261
		t = t.Headers(headers...).BorderHeader(false)
	}
	fmt.Fprintln(w, t)

	// We render the footer if present.
	if cmd.Metadata["footer"] != nil {
		fmt.Fprintln(w, headerStyle.Render(cmd.Metadata["footer"].(string)))
	}
}

// flattenState takes the state schema of each entry and flattens it into a
// schema with parent and attributes. This is done so that we can have a common
// schema for all the different types of resources. Note that this schema can be
// extremely wide and complex. There will be a unique schema for each resource
// type and, with the addition of aggregated sq, even more complexity is
// expected, doubly so if multiple providers are represented in those states.
func flattenState(resources gjson.Result, short bool) bytes.Buffer {
	flatResources := flattenStateRows(resources, short)

	jsonBytes, err := json.Marshal(flatResources)
	if err != nil {
		log.Errorf("flattenState marshal: %v", err)
		return *bytes.NewBuffer([]byte{})
	}

	raw := *bytes.NewBuffer(jsonBytes)
	return raw
}

func flattenStateRows(resources gjson.Result, short bool) []map[string]any {
	var flatResources []map[string]any

	for _, resource := range resources.Array() {
		common := getCommonFields(resource)

		instances := resource.Get("instances")
		for _, instance := range instances.Array() {
			flatResource := make(map[string]any)
			maps.Copy(flatResource, common)

			for key, value := range instance.Map() {
				flatResource[key] = value.Value()
			}

			module := ""
			if flatResource["module"] != nil {
				module = InterfaceToString(flatResource["module"]) + "."
			}

			mode := ""
			if flatResource["mode"] != "managed" {
				mode = InterfaceToString(flatResource["mode"]) + "."
			}

			indexKey := ""
			if flatResource["index_key"] != nil {
				switch v := flatResource["index_key"].(type) {
				case int, int64, float64:
					indexKey = fmt.Sprintf("[%v]", v)
				default:
					indexKey = fmt.Sprintf("[\"%v\"]", v)
				}
			}

			resourceID := fmt.Sprintf("%s%s%s.%s%s", module, mode, flatResource["type"], flatResource["name"], indexKey)
			if !short {
				re := regexp.MustCompile(`(^module.)|(.module.)`)
				resourceID = re.ReplaceAllString(resourceID, "+")
			}
			flatResource["resource"] = resourceID

			flatResources = append(flatResources, flatResource)
		}
	}

	return flatResources
}

// FlattenTerraformState flattens a full Terraform state document into a row
// set compatible with SliceDiceSpit filtering and rendering.
func FlattenTerraformState(doc []byte, short bool) ([]map[string]any, error) {
	resources := gjson.ParseBytes(doc).Get("resources")
	if !resources.Exists() {
		return []map[string]any{}, nil
	}

	return flattenStateRows(resources, short), nil
}

// getColors returns configured color values for table rendering. Each color is
// selected based on terminal background color and brightness so that we can
// make sure output is reasonably visible for all(?) terminal themes.
func getColors(key string) (header, even, odd lipgloss.TerminalColor) {
	isDark := lipgloss.HasDarkBackground()

	// Use the explicit color if found in the config and leave it up to the user
	// to choose appropriate colors for their theme. If not found, pick a
	// reasonable default based on terminal background.
	resolveColor := func(key string, light string, dark string) lipgloss.TerminalColor {
		colorCfg, err := config.GetString(key)
		if err == nil {
			return lipgloss.Color(colorCfg)
		}

		if isDark {
			return lipgloss.Color(dark)
		}
		return lipgloss.Color(light)
	}

	header = resolveColor(key+".title", "#b08800", "#f6be00")
	even = resolveColor(key+".even", "#333333", "#ffffff")
	odd = resolveColor(key+".odd", "#0088a0", "#00c8f0")

	return
}

// getCommonFields extracts common fields from a resource, excluding instances.
func getCommonFields(resource gjson.Result) map[string]any {
	common := make(map[string]any)
	for key, value := range resource.Map() {
		if key != "instances" {
			common[key] = value.Value()
		}
	}
	return common
}

// WriteIntoFiles writes the provided data into files specified by the
// --json-into and --yaml-into flags, if present. There is no check to know if
// the output file can be successfully written or if it already exists. In the
// latter case, the file will be overwritten.
func writeIntoFiles(cmd *cli.Command, data any) {
	if path2 := cmd.String("json-into"); path2 != "" {
		jsonOutput, err := json.Marshal(data)
		if err != nil {
			log.Errorf("SliceDiceSpit json marshal for json-into: %v", err)
			return
		}

		if err := os.WriteFile(path2, jsonOutput, 0o600); err != nil {
			log.Errorf("SliceDiceSpit write file for json-into: %v", err)
		}
	}

	if path2 := cmd.String("yaml-into"); path2 != "" {
		yamlOutput, err := yaml.Marshal(data)
		if err != nil {
			log.Errorf("SliceDiceSpit yaml marshal for yaml-into: %v", err)
			return
		}

		if err := os.WriteFile(path2, yamlOutput, 0o600); err != nil {
			log.Errorf("SliceDiceSpit write file for yaml-into: %v", err)
		}
	}
}
