package main

import (
	"os"
	"strings"
)

func ExampleGenerateAst() {

	input := `Feature: Minimal
 
  Scenario: minimalistic
    Given the minimalism
`
	reader := strings.NewReader(input)
	writer := os.Stdout
	GenerateTokens(reader, writer)

	// Output:
	// (1:1)FeatureLine:Feature/Minimal/
	// (2:2)Empty://
	// (3:3)ScenarioLine:Scenario/minimalistic/
	// (4:5)StepLine:Given /the minimalism/
	// EOF
}
