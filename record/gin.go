package autodoc

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// ginResponseRecorder writes to both a ResponseRecorder and the original ResponseWriter
type ginResponseRecorder struct {
	gin.ResponseWriter
	recorder     *httptest.ResponseRecorder
	closeChannel chan bool
}

func (r *ginResponseRecorder) Header() http.Header {
	return r.recorder.Header()
}

func (r *ginResponseRecorder) Write(b []byte) (int, error) {
	r.recorder.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *ginResponseRecorder) WriteHeader(statusCode int) {
	// TODO: temp fix for sse
	if statusCode == -1 {
		statusCode = 200
	}
	r.recorder.WriteHeader(statusCode)
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *ginResponseRecorder) CloseNotify() <-chan bool {
	return r.closeChannel
}

func createGinResponseRecorder(w gin.ResponseWriter) *ginResponseRecorder {
	return &ginResponseRecorder{
		ResponseWriter: w,
		recorder:       httptest.NewRecorder(),
		closeChannel:   make(chan bool, 1),
	}
}

func createTestGinContext(c *gin.Context) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := createGinResponseRecorder(c.Writer)
	c.Writer = rec
	return c, rec.recorder
}

func (r *Recorder) RecordGin(h gin.HandlerFunc, opts ...RecordOptions) gin.HandlerFunc {
	return func(c *gin.Context) {
		c, rec := createTestGinContext(c)

		if c.Request.URL.Path == "" {
			p := r.Path
			re := regexp.MustCompile(`{(.*?)}`)
			matches := re.FindAllString(r.Path, -1)
			for _, m := range matches {
				p = strings.ReplaceAll(p, m, c.Param(strings.Trim(m, "{}")))
			}
			c.Request.URL.Path = p
		}

		req := c.Request.Clone(context.Background())
		body, _ := ioutil.ReadAll(req.Body)
		req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		c.Request.Body = ioutil.NopCloser(bytes.NewBuffer(body))

		h(c)

		r.record(req, rec.Result(), opts...)
	}
}
