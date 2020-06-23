package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/terraform/backend/local"
	"github.com/hashicorp/terraform/helper/logging"
	"github.com/hashicorp/terraform/tfdiags"
	"github.com/mitchellh/cli"
	"github.com/mitchellh/colorstring"
	terraspec "github.com/nhurel/terraspec/lib"
	"github.com/zclconf/go-cty/cty"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	app = kingpin.New("terraspec", "Unit test terraform config")
	// dir = app.Flag("dir", "path to terraform config dir to test").Default(".").String()
	specDir     = app.Flag("spec", "path to folder containing test cases").Default("spec").String()
	displayPlan = app.Flag("display-plan", "Print the full plan before the results").Default("false").Bool()
)

type testCase struct {
	dir          string
	variableFile string
	specFile     string
}

func (tc *testCase) name() string {
	return filepath.Base(tc.dir)
}

type testReport struct {
	name   string
	plan   string
	report tfdiags.Diagnostics
}

func main() {
	kingpin.MustParse(app.Parse(os.Args[1:]))

	testCases := findCases(*specDir)
	if len(testCases) == 0 {
		log.Fatal("No test case found")
	}

	reports := make(chan *testReport)

	var wg sync.WaitGroup
	for _, tc := range testCases {
		wg.Add(1)
		go func(tc *testCase) {
			runTestCase(tc, reports)
			wg.Done()
		}(tc)
	}
	exitCode := 0
	go func() {
		wg.Wait()
		close(reports)
	}()

	for r := range reports {
		fmt.Printf("🏷  %s\n", r.name)
		if r.report.HasErrors() {
			exitCode = 1
		}
		if *displayPlan {
			fmt.Println(r.plan)
		}
		printDiags(r.report)
	}
	os.Exit(exitCode)
}

func runTestCase(tc *testCase, results chan<- *testReport) {
	// Disable terraform verbose logging except if TF_LOF is set
	logging.SetOutput()
	var planOutput string

	tfCtx, ctxDiags := terraspec.NewContext(".", tc.variableFile) // Setting a different folder works to parse configuration but not the modules :/
	if fatalReport(tc.name(), ctxDiags, planOutput, results) {
		return
	}
	plan, ctxDiags := tfCtx.Plan()
	if fatalReport(tc.name(), ctxDiags, planOutput, results) {
		return
	}

	log.SetOutput(os.Stderr)
	var stdout = &strings.Builder{}

	if *displayPlan {
		ui := &cli.BasicUi{
			Reader:      os.Stdin,
			Writer:      stdout,
			ErrorWriter: stdout,
		}
		local.RenderPlan(plan, nil, tfCtx.Schemas(), ui, &colorstring.Colorize{Colors: colorstring.DefaultColors})
		planOutput = stdout.String()
	}
	logging.SetOutput()

	spec, ctxDiags := terraspec.ReadSpec(tc.specFile, tfCtx.Schemas())
	if fatalReport(tc.name(), ctxDiags, planOutput, results) {
		return
	}
	ctxDiags, err := spec.Validate(plan)
	if err != nil {
		// TODO manage this error by returning a report with an error diagnostic
		log.Fatal(err)
	}
	results <- &testReport{name: tc.name(), report: ctxDiags, plan: planOutput}
}

func findCases(rootDir string) []*testCase {
	testCases := make([]*testCase, 0)

	rootFis, err := ioutil.ReadDir(rootDir)
	if err != nil {
		log.Fatal(err)
	}

	for _, rootFi := range rootFis {
		if !rootFi.IsDir() {
			continue
		}
		if testCase := findCase(filepath.Join(rootDir, rootFi.Name())); testCase != nil {
			testCases = append(testCases, testCase)
		}
	}
	if testCase := findCase(rootDir); testCase != nil {
		testCases = append(testCases, testCase)
	}
	return testCases
}

func findCase(rootDir string) *testCase {
	fis, err := ioutil.ReadDir(rootDir)
	if err != nil {
		return nil
	}
	var varFile, specFile string
	for _, fi := range fis {
		if fi.IsDir() {
			continue
		}
		if filepath.Ext(fi.Name()) == ".tfvars" {
			varFile = filepath.Join(rootDir, fi.Name())
		}
		if filepath.Ext(fi.Name()) == ".tfspec" {
			specFile = filepath.Join(rootDir, fi.Name())
		}
	}
	if specFile != "" {
		return &testCase{dir: rootDir, variableFile: varFile, specFile: specFile}
	}
	return nil
}

func fatalReport(name string, err tfdiags.Diagnostics, plan string, reports chan<- *testReport) bool {
	if err.HasErrors() {
		reports <- &testReport{name: name, report: err, plan: plan}
		return true
	}
	return false
}

func printDiags(ctxDiags tfdiags.Diagnostics) {
	for _, diag := range ctxDiags {
		switch d := diag.(type) {
		case *terraspec.TerraspecDiagnostic:
			if diag.Severity() == terraspec.Info {
				fmt.Print(" ✔  ")
			} else {
				fmt.Print(" ❌  ")
			}
			if path := tfdiags.GetAttribute(d.Diagnostic); path != nil {
				colorstring.Printf("[bold]%s ", formatPath(path))
			}
			if diag.Severity() == terraspec.Info {
				colorstring.Printf("= [green]%s\n", diag.Description().Detail)
			} else {
				colorstring.Printf(": [red]%s\n", diag.Description().Detail)

			}

		default:
			if subj := diag.Source().Subject; subj != nil {
				colorstring.Printf("[bold]%s#%d,%d : ", subj.Filename, subj.Start.Line, subj.Start.Column)
			}

			if diag.Description().Summary != "" {
				colorstring.Printf("[red]%s : ", diag.Description().Summary)
			}
			colorstring.Printf("[red]%s\n", diag.Description().Detail)

		}
	}
}

func formatPath(path cty.Path) string {
	sb := strings.Builder{}
	for i, pa := range path {
		switch p := pa.(type) {
		case cty.GetAttrStep:
			if i > 0 {
				sb.WriteRune('.')
			}
			sb.WriteString(p.Name)
		case cty.IndexStep:
			sb.WriteRune('[')
			val, _ := p.Key.AsBigFloat().Int64()
			sb.WriteString(strconv.Itoa(int(val)))
			sb.WriteRune(']')
		}
	}
	return sb.String()
}
