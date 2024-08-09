package main

import (
	"github.com/urfave/cli/v2"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/rego"
	
	"encoding/json"
	"errors"
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

type File struct {
	Name string;
	Contents string;
}

type Info struct {
	Files []File;
	Data map[string]interface{}
	Input interface{}
	Query string
}

func load(cCtx *cli.Context) (Info, error) {
	var info Info
	
	for _, file := range cCtx.StringSlice("data") {
		contents, err := os.ReadFile(file)
		if strings.HasSuffix(file, "json") {
			if err := json.Unmarshal(contents, &info.Data); err != nil {
				return info, err
			}
			continue
		}
		if err != nil {
			return info, err
		}
		info.Files = append(info.Files, File { Name: file, Contents: string(contents)})
	}

	args := cCtx.Args()
	if !args.Present() || args.Get(1) != "" {
		return info, errors.New("exactly 1 query must be specified")
	}
	info.Query = args.Get(0)

	inputFile := cCtx.String("input")
	if inputFile != "" {
		inputJson, err := os.ReadFile(inputFile)
		if err != nil {
			return info, err
		}

		if err := json.Unmarshal(inputJson, &info.Input); err != nil {
			return info, err
		}
	}

	return info, nil
}

func makeRego(info *Info) (*rego.PreparedEvalQuery, error) {
	ctx := context.Background()
	var options []func (r * rego.Rego)

	modules := make(map[string]string)
	for _, file := range info.Files {
		//d := rego.Module(file.Name, string(file.Contents))
		//options = append(options, d)
		modules[file.Name] = string(file.Contents)
	}
	compiler, err := ast.CompileModules(modules)
	if err != nil {
		return nil, err
	}
		options = append(options, rego.Compiler(compiler))

	if info.Data != nil {
		store := inmem.NewFromObject(info.Data)
		options = append(options, rego.Store(store))
	}

	options = append(options, rego.Query(info.Query))
	r := rego.New(options...)
	query, err := r.PrepareForEval(ctx)
	query.Eval(ctx)
	return &query, err
}

func profile(info *Info, query *rego.PreparedEvalQuery, show bool) (int64, error) {
	ctx := context.Background()
	rs, err := query.Eval(ctx, rego.EvalInput(nil))
	if err != nil {
		return 0, err
	}
	start := time.Now()
	rs, err = query.Eval(ctx, rego.EvalInput(info.Input))
	if err != nil {
		return 0, err
	}

	elapsed := time.Since(start).Microseconds()
	
	if show {
		rsJson, err := json.MarshalIndent(rs, "", "  ")
		if err != nil {
			return 0, err
		}			
		fmt.Println("", string(rsJson))
	}

	return elapsed, nil
}

func action(cCtx *cli.Context) error {
	numIterations := cCtx.Int("num-iterations")
	if numIterations == 0 {
		return errors.New("num-iterations must be specified")
	}

	info, err := load(cCtx)
	if err != nil {
		return err
	}

	query, err := makeRego(&info)
	if err != nil {
		return err
	}

	fresh := cCtx.Bool("fresh-query")
	totalTime := int64(0)
	for i := 0; i < numIterations; i++ {
		if fresh {
			query, err = makeRego(&info)
			if err != nil {
				return err
			}
		}
	
		elapsed, err := profile(&info, query, (i + 1) == numIterations && cCtx.Bool("show-output"))
		if err != nil {
			return err
		}
		totalTime += elapsed
	}
	average := float64(totalTime) / float64(numIterations)
	fmt.Printf("average eval time = %.2f microseconds\n", average)
	return nil
		
}

func main() {
	app := cli.NewApp()
	app.Name = "opa-profile"
	app.Description = "Profile OPA execution time"
	app.Flags = []cli.Flag{
		&cli.StringSliceFlag{
			Name:  "data",
			Aliases: []string{"d"},
			Usage: "Rego policy or data json",
		},
		&cli.StringFlag{
			Name:  "input",
			Aliases: []string{"i"},
			Usage: "input json",
		},
		&cli.IntFlag{
			Name: "num-iterations",
			Aliases: []string{"n"},
			Usage: "numer of iterations",
		},
		&cli.IntFlag{
			Name: "show-output",
			Aliases: []string{"s"},
			Usage: "show eval output",

		},
		&cli.BoolFlag{
			Name: "fresh-query",
			Aliases: []string{"f"},
			Usage: "use fresh query in each iteration",
		},
	}
	app.Action = action

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%v", err)
	}
}
