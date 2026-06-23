// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/apex/log"
)

// schemaTag represents a discovered struct field tag used when emitting schema
// information (--schema flag).
type schemaTag struct {
	Kind     string
	Name     string
	Encoding string
}

// print renders the tag into its display form.
func (t schemaTag) print() string {
	parts := []string{}
	if t.Name != "" {
		parts = append(parts, t.Name)
	}
	output := strings.Join(parts, ",")
	return output
}

// maxSchemaDepth limits the depth of schema walking to prevent infinite
// recursion.
const maxSchemaDepth = 1

// DumpSchema writes a sorted list of attribute tags for the provided type
// to the provided writer. If w is nil, os.Stdout is used.
func DumpSchema(prefix string, typ reflect.Type, w io.Writer) {
	if w == nil {
		w = os.Stdout
	}

	fmt.Fprintln(w,
		`Resource level attributes that are directly available to the --attrs flag.
For a complete schema, including relationships, use --output=raw and see the
attrs help in the documentation or man tfquery-attrs.`)
	fmt.Fprintln(w, "")

	tags := dumpSchemaWalker(prefix, typ, 0)
	if len(tags) == 0 {
		log.Debugf("No tags found for type: %s", typ.Name())
		return
	}

	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Kind == tags[j].Kind {
			return tags[i].Name < tags[j].Name
		}
		return tags[i].Kind < tags[j].Kind
	})

	for _, tag := range tags {
		fmt.Fprintln(w, tag.Name)
	}
}

// dumpSchemaWalker recursively walks a struct type discovering jsonapi tags.
func dumpSchemaWalker(holder string, typ reflect.Type, depth int) []schemaTag {
	tags := []schemaTag{}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		log.Debugf("field: %s, type: %s in %s", field.Name, field.Type, field.PkgPath)

		tagValue, ok := field.Tag.Lookup("jsonapi")
		if !ok {
			continue
		}

		tag := NewTag(holder, tagValue)
		if tag.Kind != "attr" {
			continue
		}

		tags = append(tags, tag)

		if depth < maxSchemaDepth {
			switch field.Type.Kind() {
			case reflect.Struct:
				tags = append(tags, dumpSchemaWalker(tag.Name, field.Type, depth+1)...)
			case reflect.Pointer:
				if field.Type.Elem().Kind() == reflect.Struct {
					holder := tag.Name
					if tag.Kind == "relation" {
						holder = fmt.Sprintf(".relationships.%s.data", tag.Name)
					}
					tags = append(tags, dumpSchemaWalker(holder, field.Type.Elem(), depth+1)...)
				}
			default:
				if strings.Contains(field.Type.String(), ".") {
					continue
				}
				log.Debugf("Presumed primitive field type: %s for %v", field.Type.Kind(), tag)
			}
		}
	}

	return tags
}
