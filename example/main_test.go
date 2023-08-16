package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	autodoc "github.com/arpinfidel/autodoc/record"
	"github.com/fatih/structs"
	"github.com/gin-gonic/gin"
)

type testOpt func(c *gin.Context)

func createTestContext(opts ...testOpt) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		URL:           &url.URL{},
		MultipartForm: &multipart.Form{},
		TLS:           &tls.ConnectionState{},
		Response:      &http.Response{},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, w
}

func withMethod(method string) testOpt {
	return func(c *gin.Context) {
		c.Request.Method = method
	}
}

func withBody(body interface{}) testOpt {
	return func(c *gin.Context) {
		if c.Request.Header == nil {
			c.Request.Header = make(http.Header)
		}

		withHeaders(map[string][]string{
			"Content-Type": {"application/json"},
		})

		b, _ := json.Marshal(body)
		c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(b))
	}
}

func withHeaders(headers map[string][]string) testOpt {
	return func(c *gin.Context) {
		if c.Request.Header == nil {
			c.Request.Header = make(http.Header)
		}

		for k, v := range headers {
			c.Request.Header[k] = v
		}
	}
}

func withQuery(query url.Values) testOpt {
	return func(c *gin.Context) {
		c.Request.URL.RawQuery = query.Encode()
	}
}

func withParams(params map[string]string) testOpt {
	return func(c *gin.Context) {
		for k, v := range params {
			c.Params = append(c.Params, gin.Param{
				Key:   k,
				Value: v,
			})
		}
	}
}

func withForm(data interface{}) testOpt {
	form := url.Values{}

	switch data.(type) {
	case map[string]string:
		for k, v := range data.(map[string]string) {
			form.Set(k, v)
		}
	default:
		s := structs.New(data)
		for _, field := range s.Fields() {
			fk := strings.ReplaceAll(field.Tag("json"), ",omitempty", "")
			form.Set(fk, fmt.Sprint(field.Value()))
		}

		return func(c *gin.Context) {
			if c.Request.Header == nil {
				c.Request.Header = make(http.Header)
			}

			c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			c.Request.ContentLength = int64(len(form.Encode()))
			c.Request.Body = ioutil.NopCloser(bytes.NewBufferString(form.Encode()))
		}
	}

	return func(c *gin.Context) {
		c.Request.PostForm = form
	}
}

func TestExampleFormHandler(t *testing.T) {
	recorder := autodoc.Recorder{
		Path:   "/api/v1/example",
		Method: "post",
		Tag:    "Example",
	}
	type args struct {
		statusCode int
		form       interface{}
		resp       interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Test Example",
			args: args{
				statusCode: 200,
				form: ExampleRequest{
					ID:          "id-exampple",
					Name:        "name-example",
					Description: "description-example",
				},
				resp: gin.H{"message": "success"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := createTestContext(withForm(tt.args.form))
			c.Request.Method = "POST"
			if tt.args.form != nil {

			}

			recorder.RecordGin(ExampleFormHandler(tt.args.statusCode, tt.args.resp), autodoc.RecordOptions{
				UseAsRequestExample: true,
			})(c)

			recorder.GenerateFile()
		})
	}
}
