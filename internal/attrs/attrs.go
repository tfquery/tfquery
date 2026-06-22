// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package attrs

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/tfquery/tfquery/internal/log"
)

// Attr represents each of the keys to be included in the output. These are
// typically identified by the JSON attributes key, thus the name.
type Attr struct {
	// Input configuration
	Key string `yaml:"key" json:"Key"` // The JSON key to extract from the result JSON object

	// Output configuration
	Include       bool   `yaml:"include" json:"Include"`             // Should this Attr be included in output or is it just intended for filtering and sorting?
	OutputKey     string `yaml:"outputKey" json:"OutputKey"`         // The key to use in the output. This is also used as the column title when output=text
	TransformSpec string `yaml:"transformSpec" json:"TransformSpec"` // Transformation spec to apply to the output value
}

// Transform applies the attribute's transform spec to a value and returns the
// transformed result.
func (a *Attr) Transform(value interface{}) interface{} {
	// TODO Currently only string values can be transformed.
	result, ok := value.(string)
	if !ok {
		if mapValue, ok := value.(map[string]interface{}); ok {
			log.Tracef("map value: value=%v", value)
			return mapValue
		}
		log.Tracef("non-string value: key=%s outputKey=%s value=%v", a.Key, a.OutputKey, value)
		return value
	}

	log.Tracef("transform start: key=%s outputKey=%s spec=%s value=%s", a.Key, a.OutputKey, a.TransformSpec, result)

	// Convert UTC time to local or time ago.
	if strings.ContainsAny(a.TransformSpec, "tT") {
		loc := time.Now().Location()
		if loc != nil {
			t, err := time.Parse(time.RFC3339, result)
			if err == nil {
				local := t.In(loc)
				if strings.Contains(a.TransformSpec, "T") {
					result = humanize.Time(local)
					log.Tracef("time ago: result=%s", result)
				} else {
					result = local.Format("2006-01-02T15:04:05MST")
					log.Tracef("time local: result=%s", result)
				}
			} else {
				log.Tracef("time skip: key=%s value=%s err=%v", a.Key, result, err)
			}
		}
	}

	// We need to know which case transformation appears last. This covers the
	// case where there has been a global case transformation prepended to the
	// attrs transformation and allows the attr's to carry more weight.
	// IOW... --attrs '*::U,name::l' will be lower case.
	lastL := strings.LastIndexAny(a.TransformSpec, "lL")
	lastU := strings.LastIndexAny(a.TransformSpec, "uU")

	log.Tracef("case analysis: key=%s spec=%s lastL=%d lastU=%d", a.Key, a.TransformSpec, lastL, lastU)

	if lastL > lastU {
		result = strings.ToLower(result)
		log.Tracef("case lower: result=%s", result)
	} else if lastU > lastL {
		result = strings.ToUpper(result)
		log.Tracef("case upper: result=%s", result)
	}

	// Is it a length-based transformation?
	if a.TransformSpec != "" {
		re := regexp.MustCompile(`-?\d+`)
		// Same logic as above re: case. This allows a more specific length
		// transformation to override a global one.
		match := re.FindAllString(a.TransformSpec, -1)
		if len(match) > 0 {
			// Take the last (overriding) match.
			l, _ := strconv.Atoi(match[len(match)-1])
			abs := int(math.Abs(float64(l)))
			if len(result) > abs {
				if l < 0 {
					lr := abs/2 - 1
					left := result[0:lr]
					right := result[len(result)-lr:]
					result = left + ".." + right
					log.Tracef("length middle: result=%s", result)
				} else {
					result = result[:l]
					log.Tracef("length trunc: result=%s", result)
				}
			}
		}
	}

	log.Tracef("transform done: key=%s outputKey=%s result=%s", a.Key, a.OutputKey, result)
	return result
}

// AttrList is a collection of Attr used to shape output fields.
type AttrList []Attr

// Set parses each spec from --attrs and adds it to the AttrList.
// NOTE The current implementation always returns nil. This may be changed to be
// more meaningful in the future.
func (a *AttrList) Set(value string) error {
	if value == "" || value == "*" {
		log.Debugf("early return: value=%s", value)
		return nil
	}

	const (
		jsonIdx = iota
		outputIdx
		transformIdx
	)

	// Multiple attrs specs can be included by --attrs and are separated by
	// commas.
	specs := strings.Split(value, ",")
	log.Debugf("specs split: specs=%v", specs)

specloop:
	for _, spec := range specs {
		// Default to including the attribute, assuming it is a child of the
		// .attributes key of the JSON object.
		attr := Attr{
			Include: true,
		}

		// There are three ':' delimited fields in each spec. The first is the key
		// to extract from the JSON object. The second is the key to use in the
		// output. The third is the transformation spec to apply to the output
		// value. The latter two are optional. The output key defaults to the last
		// section of the JSON key.
		fields := strings.Split(spec, ":")

		// The first field is the key to extract from the JSON payload. If it
		// begins with a !, it is excluded from the output and may, instead, be used
		// to build the value of an actual output field or for filtering and
		// sorting. If it is *, it is a global transform spec and applies to all
		// fields.
		attr.Key = strings.TrimSpace(fields[jsonIdx])
		if strings.HasPrefix(attr.Key, "!") {
			attr.Include = false
			attr.Key = attr.Key[1:]
		}

		if attr.Key == "*" {
			attr.Include = false
		}
		log.Tracef("key parsed: key=%s, include=%v", attr.Key, attr.Include)

		// Fix up the output field. If there is only one field, it is the JSON
		// extract key and the output key becomes the last segment of the
		// . notation.
		if len(fields) == 1 {
			segments := strings.Split(attr.Key, ".")
			attr.OutputKey = segments[len(segments)-1]
		} else {
			if fields[outputIdx] != "" {
				attr.OutputKey = strings.TrimSpace(fields[outputIdx])
			} else {
				attr.OutputKey = attr.Key
			}
		}
		log.Tracef("output set: outputKey=%s", attr.OutputKey)

		attr.TransformSpec = ""
		if len(fields) > transformIdx {
			attr.TransformSpec = strings.TrimSpace(fields[transformIdx])
			attr.TransformSpec = strings.TrimSuffix(attr.TransformSpec, ",")
		}
		log.Tracef("transform set: spec=%s", attr.TransformSpec)

		// If the attr already exists in the list (because it is a default for
		// a command or the user double-entered it), apply the OutputKey, Include
		// and TransformSpec to the existing Attr.
		for i := range *a {
			if (*a)[i].Key == attr.Key || (*a)[i].OutputKey == attr.Key {
				(*a)[i].Include = attr.Include
				(*a)[i].OutputKey = attr.OutputKey
				(*a)[i].TransformSpec = attr.TransformSpec
				log.Tracef("existing updated: i=%d", i)
				continue specloop
			}
		}

		// Fix up the key field. If it begins with '.', we are working off the root
		// of the JSON payload. If it does not, we are working off the .attributes
		// of the objects in the payload.
		if strings.HasPrefix(attr.Key, ".") {
			attr.Key = attr.Key[1:]
		} else if attr.Key != "*" {
			attr.Key = "attributes." + attr.Key
		}
		log.Tracef("key fixed: key=%s", attr.Key)

		*a = append(*a, attr)
		log.Tracef("attr appended: len=%d", len(*a))
	}

	return nil
}

// SetGlobalTransformSpec inserts a global transform spec at the front of all
// attrs in the list.
func (a *AttrList) SetGlobalTransformSpec() error {
	spec := ""

	// Find the global transform spec. If there is more than one, take the first.
	for attr := range *a {
		if (*a)[attr].Key == "*" {
			spec = (*a)[attr].TransformSpec
			break
		}
	}
	log.Debugf("global spec: spec=%s", spec)

	// Return early if there is no global transform spec.
	if spec == "" {
		log.Debugf("no global spec")
		return nil
	}

	// Prepend the global spec onto each attribute's spec, omitting the separator
	// if the attr's current spec is empty.
	for attr := range *a {
		if (*a)[attr].TransformSpec == "" {
			(*a)[attr].TransformSpec = spec
		} else {
			(*a)[attr].TransformSpec = spec + "," + (*a)[attr].TransformSpec
		}
	}
	log.Debugf("specs prepended")

	return nil
}

// String returns a string representation of the AttrList. This matches the
// format of the original --attrs flag.
func (a *AttrList) String() string {
	result := make([]string, 0, len(*a))
	for _, attr := range *a {
		result = append(result, fmt.Sprintf("%s:%s:%s", attr.Key, attr.OutputKey, attr.TransformSpec))
	}

	resultStr := strings.Join(result, ",")
	log.Debugf("string built: result=%s", resultStr)
	return resultStr
}

// Type returns the flag type for use with the flag.Value interface.
func (a *AttrList) Type() string { return "list" }
