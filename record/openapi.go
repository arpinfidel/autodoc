package autodoc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/google/martian/har"
	"gopkg.in/yaml.v3"
)

type OpenAPIConfig struct {
	Info       map[string]string        `yaml:"info"      `
	Components map[string]interface{}   `yaml:"components"`
	Security   []map[string]interface{} `yaml:"security"  `
	Servers    []map[string]string      `yaml:"servers"   `
}

type OpenAPI struct {
	OpenAPIConfig `yaml:",inline"`
	OpenAPI       string                 `yaml:"openapi"`
	Paths         map[string]interface{} `yaml:"paths"`
}

type RequestBody struct {
	Content map[string]Content `yaml:"content"`
}

type Content struct {
	Schema   Schema             `yaml:"schema"`
	Examples map[string]Example `yaml:"examples"`
}

type Schema struct {
	Type       string      `yaml:"type"`
	Properties interface{} `yaml:"properties"`
}

type Example struct {
	Summary string      `yaml:"summary"`
	Value   interface{} `yaml:"value"`
}

func (o *OpenAPI) Bytes() []byte {
	y, _ := yaml.Marshal(o)
	return y
}

func (o *OpenAPI) String() string {
	return string(o.Bytes())
}

func (r *Recorder) OpenAPI() OpenAPI {
	requestBody := RequestBody{}
	reqs := []har.Request{}
	i := 0
	for _, rec := range r.Records {
		if rec.Options.ExcludeFromOpenAPI {
			continue
		}
		if !rec.Options.UseAsRequestExample {
			continue
		}
		i++
		reqs = append(reqs, *rec.Request)

		req := rec.Request

		if req.PostData != nil {
			if requestBody.Content == nil {
				requestBody.Content = map[string]Content{}
			}
			// TODO: merge json
			content := requestBody.Content[getContentType(req.Headers)]

			content.Schema = Schema{
				Type:       "object",
				Properties: getJSONSchema([]byte(req.PostData.Text)),
			}

			if content.Examples == nil {
				content.Examples = map[string]Example{}
			}

			content.Examples[fmt.Sprintf("%d. %s", i, rec.Options.RequestName)] = Example{
				Summary: rec.Options.RequestSummary,
				Value:   getJSON([]byte(req.PostData.Text)),
			}

			requestBody.Content[getContentType(req.Headers)] = content
		}
	}

	params := []map[string]interface{}{}

	req := reqs[len(reqs)-1]

	// TODO: support multiple request examples
	recP := strings.Split(r.Path, "/")
	reqP := strings.Split(req.URL, "?")[0]
	reqPs := strings.Split(reqP, "/")
	if len(recP) != len(reqPs) {
		fmt.Println("request path does not match recorder path. skipping path parsing")
	} else {
		for i := range recP {
			recP := recP[i]
			reqP := reqPs[i]
			if recP == reqP {
				continue
			}

			params = append(params, map[string]interface{}{
				"in":       "path",
				"name":     strings.Trim(recP, "{}"),
				"required": true,
				"schema": map[string]interface{}{
					"type": "string",
				},
				"example": reqP,
			})
		}
	}

	for _, q := range req.QueryString {
		p := map[string]interface{}{
			"in":   "query",
			"name": q.Name,
		}

		p["schema"] = map[string]interface{}{
			"type": "string",
		}
		p["example"] = q.Value

		params = append(params, p)
	}

	for _, h := range req.Headers {
		p := map[string]interface{}{
			"in":       "header",
			"name":     h.Name,
			"required": true,
		}

		p["schema"] = map[string]interface{}{
			"type": "string",
		}
		p["example"] = h.Value

		params = append(params, p)
	}

	responses := map[string]interface{}{}
	for _, rec := range r.Records {
		if rec.Options.ExcludeFromOpenAPI {
			continue
		}
		responses[strconv.Itoa(rec.Response.Status)] = map[string]interface{}{
			"description": rec.Options.ResponseDescription,
			"content": map[string]interface{}{
				getContentType(rec.Response.Headers): map[string]interface{}{
					"schema": map[string]interface{}{
						"type":       "object",
						"properties": getJSONSchema(rec.Response.Content.Text),
					},
				},
			},
		}
	}

	yml := OpenAPI{
		OpenAPI: "3.0.3",
		OpenAPIConfig: OpenAPIConfig{
			Info: map[string]string{
				"title":   "",
				"version": "1.0.0",
			},
		},
		Paths: map[string]interface{}{
			r.Path: map[string]interface{}{
				r.Method: map[string]interface{}{
					"tags":        []string{r.Tag},
					"description": r.APIDescription,
					"summary":     r.APISummary,
					"requestBody": requestBody,
					"parameters":  params,
					"responses":   responses,
				},
			},
		},
	}
	return yml
}
