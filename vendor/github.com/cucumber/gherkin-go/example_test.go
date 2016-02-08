package gherkin_test

import (
	"fmt"
	"os"
	"strings"

	gherkin "."
)

func ExampleParseFeature() {

	input := `Feature: Tagged Examples

  Scenario Outline: minimalistic
    Given the <what>

    @foo
    Examples:
      | what |
      | foo  |

    @bar
    Examples:
      | what |
      | bar  |

  @zap
  Scenario: ha ok
`
	r := strings.NewReader(input)

	feature, err := gherkin.ParseFeature(r)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stdout, "Location: %+v\n", feature.Location)
	fmt.Fprintf(os.Stdout, "Keyword: %+v\n", feature.Keyword)
	fmt.Fprintf(os.Stdout, "Name: %+v\n", feature.Name)
	fmt.Fprintf(os.Stdout, "ScenarioDefinitions: length: %+v\n", len(feature.ScenarioDefinitions))

	scenario1, _ := feature.ScenarioDefinitions[0].(*gherkin.ScenarioOutline)
	fmt.Fprintf(os.Stdout, " 1: Location: %+v\n", scenario1.Location)
	fmt.Fprintf(os.Stdout, "    Keyword: %+v\n", scenario1.Keyword)
	fmt.Fprintf(os.Stdout, "    Name: %+v\n", scenario1.Name)
	fmt.Fprintf(os.Stdout, "    Steps: length: %+v\n", len(scenario1.Steps))

	scenario2, _ := feature.ScenarioDefinitions[1].(*gherkin.Scenario)
	fmt.Fprintf(os.Stdout, " 2: Location: %+v\n", scenario2.Location)
	fmt.Fprintf(os.Stdout, "    Keyword: %+v\n", scenario2.Keyword)
	fmt.Fprintf(os.Stdout, "    Name: %+v\n", scenario2.Name)
	fmt.Fprintf(os.Stdout, "    Steps: length: %+v\n", len(scenario2.Steps))

	// Output:
	//
	// Location: &{Line:1 Column:1}
	// Keyword: Feature
	// Name: Tagged Examples
	// ScenarioDefinitions: length: 2
	//  1: Location: &{Line:3 Column:3}
	//     Keyword: Scenario Outline
	//     Name: minimalistic
	//     Steps: length: 1
	//  2: Location: &{Line:17 Column:3}
	//     Keyword: Scenario
	//     Name: ha ok
	//     Steps: length: 0
	//
}

func ExampleParseMultipleFeatures() {

	builder := gherkin.NewAstBuilder()
	parser := gherkin.NewParser(builder)
	parser.StopAtFirstError(false)
	matcher := gherkin.NewMatcher(gherkin.GherkinDialectsBuildin())

	input1 := `Feature: Test`
	r1 := strings.NewReader(input1)

	err1 := parser.Parse(gherkin.NewScanner(r1), matcher)
	if err1 != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err1)
		return
	}
	feature1 := builder.GetFeature()
	fmt.Fprintf(os.Stdout, "Location: %+v\n", feature1.Location)
	fmt.Fprintf(os.Stdout, "Keyword: %+v\n", feature1.Keyword)
	fmt.Fprintf(os.Stdout, "Name: %+v\n", feature1.Name)
	fmt.Fprintf(os.Stdout, "ScenarioDefinitions: length: %+v\n", len(feature1.ScenarioDefinitions))
	fmt.Fprintf(os.Stdout, "\n")

	input2 := `Feature: Test2`
	r2 := strings.NewReader(input2)

	err2 := parser.Parse(gherkin.NewScanner(r2), matcher)
	if err2 != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err2)
		return
	}
	feature2 := builder.GetFeature()
	fmt.Fprintf(os.Stdout, "Location: %+v\n", feature2.Location)
	fmt.Fprintf(os.Stdout, "Keyword: %+v\n", feature2.Keyword)
	fmt.Fprintf(os.Stdout, "Name: %+v\n", feature2.Name)
	fmt.Fprintf(os.Stdout, "ScenarioDefinitions: length: %+v\n", len(feature2.ScenarioDefinitions))


	// Output:
	//
	// Location: &{Line:1 Column:1}
	// Keyword: Feature
	// Name: Test
	// ScenarioDefinitions: length: 0
	//
	// Location: &{Line:1 Column:1}
	// Keyword: Feature
	// Name: Test2
	// ScenarioDefinitions: length: 0
	//
}

func ExampleParseFeatureAfterParseError() {

	builder := gherkin.NewAstBuilder()
	parser := gherkin.NewParser(builder)
	parser.StopAtFirstError(false)
	matcher := gherkin.NewMatcher(gherkin.GherkinDialectsBuildin())

	input1 := `# a comment
Feature: Foo
  Scenario: Bar
    Given x
` + "      ```" + `
      unclosed docstring`
	r1 := strings.NewReader(input1)

	err1 := parser.Parse(gherkin.NewScanner(r1), matcher)
	if err1 != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err1)
	}
	fmt.Fprintf(os.Stdout, "\n")

	input2 := `Feature: Foo
  Scenario: Bar
    Given x
      """
      closed docstring
      """`
	r2 := strings.NewReader(input2)

	err2 := parser.Parse(gherkin.NewScanner(r2), matcher)
	if err2 != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err2)
		return
	}
	feature2 := builder.GetFeature()
        fmt.Fprintf(os.Stdout, "Comments: length: %+v\n", len(feature2.Comments)) 
	fmt.Fprintf(os.Stdout, "Location: %+v\n", feature2.Location)
	fmt.Fprintf(os.Stdout, "Keyword: %+v\n", feature2.Keyword)
	fmt.Fprintf(os.Stdout, "Name: %+v\n", feature2.Name)
	fmt.Fprintf(os.Stdout, "ScenarioDefinitions: length: %+v\n", len(feature2.ScenarioDefinitions))
	scenario1, _ := feature2.ScenarioDefinitions[0].(*gherkin.Scenario)
	fmt.Fprintf(os.Stdout, " 1: Location: %+v\n", scenario1.Location)
	fmt.Fprintf(os.Stdout, "    Keyword: %+v\n", scenario1.Keyword)
	fmt.Fprintf(os.Stdout, "    Name: %+v\n", scenario1.Name)
	fmt.Fprintf(os.Stdout, "    Steps: length: %+v\n", len(scenario1.Steps))


	// Output:
	//
	// Parser errors:
        // (7:0): unexpected end of file, expected: #DocStringSeparator, #Other
	//
	// Comments: length: 0
	// Location: &{Line:1 Column:1}
	// Keyword: Feature
	// Name: Foo
	// ScenarioDefinitions: length: 1
	//  1: Location: &{Line:2 Column:3}
	//     Keyword: Scenario
	//     Name: Bar
	//     Steps: length: 1
	//
}

func ExampleChangeDefaultDialect() {

	builder := gherkin.NewAstBuilder()
	parser := gherkin.NewParser(builder)
	parser.StopAtFirstError(false)
	matcher := gherkin.NewLanguageMatcher(gherkin.GherkinDialectsBuildin(), "no")
	input := "Egenskap: i18n support"
	reader := strings.NewReader(input)

	err := parser.Parse(gherkin.NewScanner(reader), matcher)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%s\n", err)
		return
	}
	feature := builder.GetFeature()
	fmt.Fprintf(os.Stdout, "Location: %+v\n", feature.Location)
	fmt.Fprintf(os.Stdout, "Keyword: %+v\n", feature.Keyword)
	fmt.Fprintf(os.Stdout, "Name: %+v\n", feature.Name)
	fmt.Fprintf(os.Stdout, "ScenarioDefinitions: length: %+v\n", len(feature.ScenarioDefinitions))

	// Output:
	//
	// Location: &{Line:1 Column:1}
	// Keyword: Egenskap
	// Name: i18n support
	// ScenarioDefinitions: length: 0
	//
}
