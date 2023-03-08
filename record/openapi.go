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

func (o *OpenAPI) Bytes() []byte {
	y, _ := yaml.Marshal(o)
	return y
}

func (o *OpenAPI) String() string {
	return string(o.Bytes())
}

func (r *Recorder) OpenAPI() OpenAPI {
	req := har.Request{}
	reqIsFlagged := false
	for _, rec := range r.Records {
		if rec.Options.ExcludeFromOpenAPI {
			continue
		}
		if rec.Response.Status == r.ExpectedStatusCode && !reqIsFlagged {
			req = *rec.Request
		}
		if rec.Options.UseAsRequestExample {
			reqIsFlagged = true
			req = *rec.Request
		}
	}
	requestBody := map[string]interface{}{}
	if req.PostData != nil {
		content := map[string]interface{}{
			getContentType(req.Headers): map[string]interface{}{
				"schema": map[string]interface{}{
					"type":       "object",
					"properties": getJSON([]byte(req.PostData.Text)),
				},
			},
		}
		requestBody["content"] = content
	}

	params := []map[string]interface{}{}

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
			"description": rec.Options.RecordDescription,
			"content": map[string]interface{}{
				getContentType(rec.Response.Headers): map[string]interface{}{
					"schema": map[string]interface{}{
						"type":       "object",
						"properties": getJSON(rec.Response.Content.Text),
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
