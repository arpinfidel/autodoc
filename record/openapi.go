package autodoc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/martian/har"
	"gopkg.in/yaml.v3"
)

type OpenAPIConfig struct {
	Info       OpenAPIInfo              `yaml:"info"`
	Components map[string]interface{}   `yaml:"components"`
	Security   []map[string]interface{} `yaml:"security"  `
	Servers    []map[string]string      `yaml:"servers"   `
}

type OpenAPIInfo struct {
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
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

func (re *Recorder) OpenAPI() OpenAPI {
	requestBody := RequestBody{}
	reqs := []har.Request{}
	i := 0
	formData := []har.Param{}
	for _, rec := range re.Records {
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

			content := requestBody.Content[getContentType(req.Headers)]
			content.Schema = Schema{
				Type: "object",
			}

			if content.Examples == nil {
				content.Examples = map[string]Example{}
			}

			if _, ok := content.Schema.Properties.(map[string]interface{}); !ok {
				content.Schema.Properties = map[string]interface{}{}
			}

			exampleName := fmt.Sprintf("%d. %s", i, rec.Options.RequestName)
			if rec.Options.RequestName == "" {
				exampleName = fmt.Sprintf("%d. Example", i)
			}

			switch getContentType(req.Headers) {
			case "application/json":
				content.Schema.Properties = getJSONSchema([]byte(req.PostData.Text))
				content.Examples[exampleName] = Example{
					Summary: rec.Options.RequestSummary,
					Value:   getJSON([]byte(req.PostData.Text)),
				}
			case "application/x-www-form-urlencoded":
				exampleArr := []string{}
				for _, p := range req.PostData.Params {
					if p.Name == "" {
						continue
					}

					content.Schema.Properties.(map[string]interface{})[p.Name] = map[string]interface{}{
						"type":    predictValueType(p.Value),
						"example": p.Value,
					}

					exampleArr = append(exampleArr, fmt.Sprintf("%s=%s", p.Name, p.Value))
				}

				content.Examples[exampleName] = Example{
					Summary: rec.Options.RequestSummary,
					Value:   strings.Join(exampleArr, "&"),
				}
			case "multipart/form-data":
			// TODO : handle file submission
			case "text/plain":
				content.Examples[exampleName] = Example{
					Summary: rec.Options.RequestSummary,
					Value:   req.PostData.Text,
				}
			}

			requestBody.Content[getContentType(req.Headers)] = content
			formData = append(formData, req.PostData.Params...)
		}
	}

	params := []map[string]interface{}{}

	req := reqs[len(reqs)-1]

	// TODO: support multiple request examples
	recP := strings.Split(re.Path, "/")
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
		params = append(params, map[string]interface{}{
			"in":   "query",
			"name": q.Name,
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": q.Value,
		})
	}

	for _, h := range req.Headers {
		params = append(params, map[string]interface{}{
			"in":       "header",
			"name":     h.Name,
			"required": true,
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": h.Value,
		})
	}

	responses := map[string]interface{}{}
	for _, rec := range re.Records {
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
		OpenAPI:       "3.0.3",
		OpenAPIConfig: OpenAPIConfig{},
		Paths: map[string]interface{}{
			re.Path: map[string]interface{}{
				re.Method: map[string]interface{}{
					"tags":        []string{re.Tag},
					"description": re.APIDescription,
					"summary":     re.APISummary,
					"requestBody": requestBody,
					"parameters":  params,
					"responses":   responses,
				},
			},
		},
	}
	return yml
}

var (
	matchNumber = regexp.MustCompile(`^\d+$|^\d+\.\d+$`)
	matchBool   = regexp.MustCompile(`^(true|false)$`)
	matchArray  = regexp.MustCompile(`^\[.*\]$`)
	matchObject = regexp.MustCompile(`^{.*}$`)
)

func predictValueType(val string) string {
	switch true {
	case matchBool.MatchString(val):
		return "boolean"
	case matchArray.MatchString(val):
		return "array"
	case matchObject.MatchString(val):
		return "object"
	case matchNumber.MatchString(val):
		return "number"
	default:
		return "string"
	}
}
