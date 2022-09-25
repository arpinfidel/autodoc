package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	autodoc "github.com/tkp-richard/autodoc/record"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func getFiles() (paths []string) {
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

		paths = append(paths, path)
		return nil
	})

	return paths
}

func fileToRecorder(path string) (r autodoc.Recorder, err error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return r, err
	}

	err = json.Unmarshal(f, &r)
	if err != nil {
		return r, err
	}

	return r, nil
}

func writeFile(b []byte) error {
	os.Mkdir("autodoc", os.ModePerm)
	f, err := os.Create("./autodoc/openapi.yaml")
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(b)
	return err
}

func openAPI() error {
	all := autodoc.OpenAPI{
		OpenAPI: "3.0.3",
		Info: autodoc.OpenAPIInfo{
			Title:   "",
			Version: "1.0.0",
		},
	}

	paths := getFiles()
	for _, path := range paths {
		fmt.Println("found autodoc file:", path)

		recorder, err := fileToRecorder(path)
		if err != nil {
			return err
		}

		o := recorder.OpenAPI()

		for i := range o.Paths {
			all.Paths[i] = o.Paths[i]
		}
	}

	y, err := yaml.Marshal(all)
	if err != nil {
		return err
	}

	return writeFile(y)
}

func main() {
	var (
		format string
	)
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "format",
				Value:       "openapi",
				Aliases:     []string{"f"},
				Usage:       "openapi",
				Destination: &format,
			},
		},
		Name: "autodoc",
		Action: func(*cli.Context) error {
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

	switch format {
	case "openapi":
		err := openAPI()
		if err != nil {
			panic(err)
		}
	}
}
