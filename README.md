# autodoc

Automatically generate OpenAPI documentation from unit tests

Currently only supports json request/response and path/query parameters

- import recorder
- call the record and generate functions in your test cases. this will generate temporary files
- run the test cases
- call `autodoc` in your root directory to generate the OpenAPI file containing all tests

## usage

```bash
go install github.com/arpinfidel/autodoc
```

```go
import autodoc "github.com/arpinfidel/autodoc/record"
```

```go
r := autodoc.Recorder{
  Path:   "/foo/bar",
  Method: "post",
  Tag:    "foo",
  APISummary:         "Foo Bar",
  APIDescription:     "Foo to the bar for the lorem ipsum.",
}

for _, tt := range tests {
  t.Run(tt.name, func(t *testing.T) {
    gin.SetMode(gin.TestMode)

    c, _ := gin.CreateTestContext(w)
    c.Request = &http.Request{
      URL:           &url.URL{},
      MultipartForm: &multipart.Form{},
      TLS:           &tls.ConnectionState{},
      Response:      &http.Response{},
    }

      // Foobar being a gin.HandlerFunc
    r.RecordGin(handler.FooBar, autodoc.RecordOptions{
      UseAsRequestExample: tt.isSuccessCase,
    })(c)
    
    if env == "development" {
      r.GenerateFile()
  
      // Or for standard http handler
      // r.Record(handler.FooBar)(w, r)
    }

    // test logic here
  })
}
```

```bash
autodoc
```

## todo

- [ ] response headers (recording done. just openapi left)
- [ ] form body (recording done)
- [ ] postman collection
- [ ] other body types
- [ ] multiple examples for request body
