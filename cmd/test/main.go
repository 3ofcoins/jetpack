package main

import (
	"fmt"
	"os"

	"github.com/DATA-DOG/godog"
)

func iCdTo(path string) error {
	return os.Chdir(path)
}

func anInitializedJetpackInstallation() error {
	// TODO: fix `jetpack init` to be idempotent
	return nil
	return godog.ErrPending
}

func noImageNamed(imgName string) error {
	if exists, err := isThereImage(imgName); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("There is an image named %s", imgName)
	}
	return nil
}

func anImageNamed(imgName string) error {
	if exists, err := isThereImage(imgName); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("There is no image named %s", imgName)
	}
	return nil
}

func main() {
	godog.Run(func(s *godog.Suite) {
		origWd, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		s.BeforeScenario(func(_ interface{}) {
			if err := os.Chdir(origWd); err != nil {
				panic(err)
			}
		})

		s.Step(`^an initialized Jetpack installation$`, anInitializedJetpackInstallation)
		s.Step(`^no image named "([^"]*)"$`, noImageNamed)
		s.Step(`^there is an image named "([^"]*)"$`, anImageNamed)
		s.Step(`^I cd to "([^"]*)"$`, iCdTo)
		s.Step(`^I run:\s*(.*)$`, iRun)
	})
}
