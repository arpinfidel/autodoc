# autodoc
Automatically generate OpenAPI documentation from unit tests

Currently only supports json request/response and result is only printed to stdout

# usage
```go
gin.SetMode(gin.TestMode)

c, _ := gin.CreateTestContext(w)
c.Request = &http.Request{
	URL:           &url.URL{},
	MultipartForm: &multipart.Form{},
	TLS:           &tls.ConnectionState{},
	Response:      &http.Response{},
}

r := autodoc.Recorder{
	Path:   "/foo/bar",
	Method: "post",
	Tag:    "foo",
}
// Foobar being a gin.HandlerFunc
r.RecordGin(handler.FooBar)(c)

// Or for standard http handler
// r.Record(handler.FooBar)(w, r)

r.Print()
```