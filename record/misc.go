package autodoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/martian/har"
)

func getType(i interface{}) map[string]interface{} {
	var m map[string]interface{}
	switch i := i.(type) {
	case json.Number:
		if n, err := i.Int64(); err == nil {
			m = map[string]interface{}{
				"type":    "integer",
				"example": n,
			}
		} else if n, err := i.Float64(); err == nil {
			m = map[string]interface{}{
				"type":    "number",
				"example": n,
			}
		} else {
			panic(fmt.Sprintf("unexpected type %T", i))
		}
	case string:
		m = map[string]interface{}{
			"type":    "string",
			"example": i,
		}
	case bool:
		m = map[string]interface{}{
			"type":    "boolean",
			"example": i,
		}
	case map[string]interface{}:
		m = map[string]interface{}{
			"type": "object",
		}
		p := map[string]interface{}{}
		for k, v := range i {
			p[k] = getType(v)
		}
		m["properties"] = p
	case []interface{}:
		m = map[string]interface{}{
			"type": "array",
		}
		if len(i) > 0 {
			m["items"] = getType(i[0])
		}
	case nil:
		m = map[string]interface{}{
			"example": nil,
		}
	default:
		panic(fmt.Sprintf("unexpected type %T %#v", i, i))
	}
	return m
}

func getContentType(headers []har.Header) string {
	for _, h := range headers {
		if strings.ToLower(h.Name) == "content-type" && h.Value != "" {
			return h.Value
		}
	}
	return "application/json"
}

func getJSONSchema(b []byte) map[string]interface{} {
	m := map[string]interface{}{}
	j := map[string]interface{}{}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	d.Decode(&j)
	for k, v := range j {
		m[k] = getType(v)
	}
	return m
}

func getJSON(b []byte) map[string]interface{} {
	m := map[string]interface{}{}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	d.Decode(&m)
	return m
}
