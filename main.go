package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	autodoc "github.com/tkp-richard/autodoc/record"
	"gopkg.in/yaml.v3"
)

func main() {
	var all map[string]interface{}
	filepath.Walk(".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		p := strings.Split(path, "/")
		folder := ""
		file := ""
		if len(p) > 1 {
			folder = p[len(p)-2]
			file = p[len(p)-1]
		}
		if folder != "autodoc" || !strings.HasPrefix(file, "autodoc-") {
			return nil
		}

		fmt.Println("found file", path)

		f, err := ioutil.ReadFile(path)
		if err != nil {
			panic(err)
		}

		recorder := autodoc.Recorder{}
		err = json.Unmarshal(f, &recorder)
		if err != nil {
			panic(err)
		}

		o := recorder.OpenAPI()

		if all == nil {
			all = o
		} else {
			i := o["paths"]
			m, ok := i.(map[string]interface{})
			if !ok {
				panic("invalid file format: " + path)
			}
			for p := range m {
				am := all["paths"].(map[string]interface{})
				am[p] = m[p]
			}
		}

		return nil
	})

	y, err := yaml.Marshal(all)
	if err != nil {
		panic(err)
	}

	os.Mkdir("autodoc", os.ModePerm)
	f, err := os.Create("./autodoc/openapi.yaml")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	println(string(y))

	_, err = f.Write(y)
	if err != nil {
		panic(err)
	}
}
