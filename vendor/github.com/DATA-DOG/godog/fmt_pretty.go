package godog

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/cucumber/gherkin-go"
)

func init() {
	Format("pretty", "Prints every feature with runtime statuses.", &pretty{
		basefmt: basefmt{
			started: time.Now(),
			indent:  2,
		},
	})
}

var outlinePlaceholderRegexp = regexp.MustCompile("<[^>]+>")

// a built in default pretty formatter
type pretty struct {
	basefmt
	commentPos      int
	backgroundSteps int

	// outline
	outlineSteps       []*stepResult
	outlineNumExample  int
	outlineNumExamples int
}

// a line number representation in feature file
func (f *pretty) line(loc *gherkin.Location) string {
	return cl(fmt.Sprintf("# %s:%d", f.features[len(f.features)-1].Path, loc.Line), black)
}

func (f *pretty) length(node interface{}) int {
	switch t := node.(type) {
	case *gherkin.Background:
		return f.indent + len(strings.TrimSpace(t.Keyword)+": "+t.Name)
	case *gherkin.Step:
		return f.indent*2 + len(strings.TrimSpace(t.Keyword)+" "+t.Text)
	case *gherkin.Scenario:
		return f.indent + len(strings.TrimSpace(t.Keyword)+": "+t.Name)
	case *gherkin.ScenarioOutline:
		return f.indent + len(strings.TrimSpace(t.Keyword)+": "+t.Name)
	}
	panic(fmt.Sprintf("unexpected node %T to determine length", node))
}

func (f *pretty) Feature(ft *gherkin.Feature, p string) {
	if len(f.features) != 0 {
		// not a first feature, add a newline
		fmt.Println("")
	}
	f.features = append(f.features, &feature{Path: p, Feature: ft})
	fmt.Println(bcl(ft.Keyword+": ", white) + ft.Name)
	if strings.TrimSpace(ft.Description) != "" {
		for _, line := range strings.Split(ft.Description, "\n") {
			fmt.Println(s(f.indent) + strings.TrimSpace(line))
		}
	}
	if ft.Background != nil {
		f.commentPos = f.longestStep(ft.Background.Steps, f.length(ft.Background))
		f.backgroundSteps = len(ft.Background.Steps)
		fmt.Println("\n" + s(f.indent) + bcl(ft.Background.Keyword+": "+ft.Background.Name, white))
	}
}

// Node takes a gherkin node for formatting
func (f *pretty) Node(node interface{}) {
	f.basefmt.Node(node)

	switch t := node.(type) {
	case *gherkin.Examples:
		f.outlineNumExamples = len(t.TableBody)
		f.outlineNumExample++
	case *gherkin.Scenario:
		f.commentPos = f.longestStep(t.Steps, f.length(t))
		text := s(f.indent) + bcl(t.Keyword+": ", white) + t.Name
		text += s(f.commentPos-f.length(t)+1) + f.line(t.Location)
		fmt.Println("\n" + text)
	case *gherkin.ScenarioOutline:
		f.commentPos = f.longestStep(t.Steps, f.length(t))
		text := s(f.indent) + bcl(t.Keyword+": ", white) + t.Name
		text += s(f.commentPos-f.length(t)+1) + f.line(t.Location)
		fmt.Println("\n" + text)
		f.outlineNumExample = -1
	}
}

// Summary sumarize the feature formatter output
func (f *pretty) Summary() {
	// failed steps on background are not scenarios
	var failedScenarios []*stepResult
	for _, fail := range f.failed {
		switch fail.owner.(type) {
		case *gherkin.Scenario:
			failedScenarios = append(failedScenarios, fail)
		case *gherkin.ScenarioOutline:
			failedScenarios = append(failedScenarios, fail)
		}
	}
	if len(failedScenarios) > 0 {
		fmt.Println("\n--- " + cl("Failed scenarios:", red) + "\n")
		var unique []string
		for _, fail := range failedScenarios {
			var found bool
			for _, in := range unique {
				if in == fail.line() {
					found = true
					break
				}
			}
			if !found {
				unique = append(unique, fail.line())
			}
		}

		for _, fail := range unique {
			fmt.Println("    " + cl(fail, red))
		}
	}
	f.basefmt.Summary()
}

func (f *pretty) printOutlineExample(outline *gherkin.ScenarioOutline) {
	var msg string
	clr := green

	example := outline.Examples[f.outlineNumExample]
	firstExample := f.outlineNumExamples == len(example.TableBody)
	printSteps := firstExample && f.outlineNumExample == 0

	for i, res := range f.outlineSteps {
		// determine example row status
		switch {
		case res.typ == failed:
			msg = res.err.Error()
			clr = res.typ.clr()
		case res.typ == undefined || res.typ == pending:
			clr = res.typ.clr()
		case res.typ == skipped && clr == green:
			clr = cyan
		}
		if printSteps {
			// in first example, we need to print steps
			var text string
			ostep := outline.Steps[i]
			if res.def != nil {
				if m := outlinePlaceholderRegexp.FindAllStringIndex(ostep.Text, -1); len(m) > 0 {
					var pos int
					for i := 0; i < len(m); i++ {
						pair := m[i]
						text += cl(ostep.Text[pos:pair[0]], cyan)
						text += bcl(ostep.Text[pair[0]:pair[1]], cyan)
						pos = pair[1]
					}
					text += cl(ostep.Text[pos:len(ostep.Text)], cyan)
				} else {
					text = cl(ostep.Text, cyan)
				}
				text += s(f.commentPos-f.length(ostep)+1) + cl(fmt.Sprintf("# %s", res.def.funcName()), black)
			} else {
				text = cl(ostep.Text, cyan)
			}
			// print the step outline
			fmt.Println(s(f.indent*2) + cl(strings.TrimSpace(ostep.Keyword), cyan) + " " + text)
		}
	}

	cells := make([]string, len(example.TableHeader.Cells))
	max := longest(example)
	// an example table header
	if firstExample {
		fmt.Println("")
		fmt.Println(s(f.indent*2) + bcl(example.Keyword+": ", white) + example.Name)

		for i, cell := range example.TableHeader.Cells {
			cells[i] = cl(cell.Value, cyan) + s(max[i]-len(cell.Value))
		}
		fmt.Println(s(f.indent*3) + "| " + strings.Join(cells, " | ") + " |")
	}

	// an example table row
	row := example.TableBody[len(example.TableBody)-f.outlineNumExamples]
	for i, cell := range row.Cells {
		cells[i] = cl(cell.Value, clr) + s(max[i]-len(cell.Value))
	}
	fmt.Println(s(f.indent*3) + "| " + strings.Join(cells, " | ") + " |")

	// if there is an error
	if msg != "" {
		fmt.Println(s(f.indent*4) + bcl(msg, red))
	}
}

func (f *pretty) printStep(step *gherkin.Step, def *StepDef, c color) {
	text := s(f.indent*2) + cl(strings.TrimSpace(step.Keyword), c) + " "
	switch {
	case def != nil:
		if m := (def.Expr.FindStringSubmatchIndex(step.Text))[2:]; len(m) > 0 {
			var pos, i int
			for pos, i = 0, 0; i < len(m); i++ {
				if math.Mod(float64(i), 2) == 0 {
					text += cl(step.Text[pos:m[i]], c)
				} else {
					text += bcl(step.Text[pos:m[i]], c)
				}
				pos = m[i]
			}
			text += cl(step.Text[pos:len(step.Text)], c)
		} else {
			text += cl(step.Text, c)
		}
		text += s(f.commentPos-f.length(step)+1) + cl(fmt.Sprintf("# %s", def.funcName()), black)
	default:
		text += cl(step.Text, c)
	}

	fmt.Println(text)
	switch t := step.Argument.(type) {
	case *gherkin.DataTable:
		f.printTable(t, c)
	case *gherkin.DocString:
		var ct string
		if len(t.ContentType) > 0 {
			ct = " " + cl(t.ContentType, c)
		}
		fmt.Println(s(f.indent*3) + cl(t.Delimitter, c) + ct)
		for _, ln := range strings.Split(t.Content, "\n") {
			fmt.Println(s(f.indent*3) + cl(ln, c))
		}
		fmt.Println(s(f.indent*3) + cl(t.Delimitter, c))
	}
}

func (f *pretty) printStepKind(res *stepResult) {
	// do not print background more than once
	if _, ok := res.owner.(*gherkin.Background); ok {
		switch {
		case f.backgroundSteps == 0:
			return
		case f.backgroundSteps > 0:
			f.backgroundSteps--
		}
	}

	if outline, ok := res.owner.(*gherkin.ScenarioOutline); ok {
		f.outlineSteps = append(f.outlineSteps, res)
		if len(f.outlineSteps) == len(outline.Steps) {
			// an outline example steps has went through
			f.printOutlineExample(outline)
			f.outlineSteps = []*stepResult{}
			f.outlineNumExamples--
		}
		return
	}

	f.printStep(res.step, res.def, res.typ.clr())
	if res.err != nil {
		fmt.Println(s(f.indent*2) + bcl(res.err, red))
	}
	if res.typ == pending {
		fmt.Println(s(f.indent*3) + cl("TODO: write pending definition", yellow))
	}
}

// print table with aligned table cells
func (f *pretty) printTable(t *gherkin.DataTable, c color) {
	var l = longest(t)
	var cols = make([]string, len(t.Rows[0].Cells))
	for _, row := range t.Rows {
		for i, cell := range row.Cells {
			cols[i] = cell.Value + s(l[i]-len(cell.Value))
		}
		fmt.Println(s(f.indent*3) + cl("| "+strings.Join(cols, " | ")+" |", c))
	}
}

func (f *pretty) Passed(step *gherkin.Step, match *StepDef) {
	f.basefmt.Passed(step, match)
	f.printStepKind(f.passed[len(f.passed)-1])
}

func (f *pretty) Skipped(step *gherkin.Step) {
	f.basefmt.Skipped(step)
	f.printStepKind(f.skipped[len(f.skipped)-1])
}

func (f *pretty) Undefined(step *gherkin.Step) {
	f.basefmt.Undefined(step)
	f.printStepKind(f.undefined[len(f.undefined)-1])
}

func (f *pretty) Failed(step *gherkin.Step, match *StepDef, err error) {
	f.basefmt.Failed(step, match, err)
	f.printStepKind(f.failed[len(f.failed)-1])
}

func (f *pretty) Pending(step *gherkin.Step, match *StepDef) {
	f.basefmt.Pending(step, match)
	f.printStepKind(f.pending[len(f.pending)-1])
}

// longest gives a list of longest columns of all rows in Table
func longest(tbl interface{}) []int {
	var rows []*gherkin.TableRow
	switch t := tbl.(type) {
	case *gherkin.Examples:
		rows = append(rows, t.TableHeader)
		rows = append(rows, t.TableBody...)
	case *gherkin.DataTable:
		rows = append(rows, t.Rows...)
	}

	longest := make([]int, len(rows[0].Cells))
	for _, row := range rows {
		for i, cell := range row.Cells {
			if longest[i] < len(cell.Value) {
				longest[i] = len(cell.Value)
			}
		}
	}
	return longest
}

func (f *pretty) longestStep(steps []*gherkin.Step, base int) int {
	ret := base
	for _, step := range steps {
		length := f.length(step)
		if length > ret {
			ret = length
		}
	}
	return ret
}
