[![Build Status](https://travis-ci.org/DATA-DOG/godog.svg?branch=master)](https://travis-ci.org/DATA-DOG/godog)
[![GoDoc](https://godoc.org/github.com/DATA-DOG/godog?status.svg)](https://godoc.org/github.com/DATA-DOG/godog)

# Godog

<p align="center"><img src="https://raw.github.com/DATA-DOG/godog/master/logo.png" alt="Godog logo" style="width:250px;" /></p>

**The API is likely to change a few times before we reach 1.0.0**

**Godog** is an open source behavior-driven development framework for [go][golang] programming language.
What is behavior-driven development, you ask? It’s the idea that you start by writing human-readable sentences that
describe a feature of your application and how it should work, and only then implement this behavior in software.

The project is inspired by [behat][behat] and [cucumber][cucumber] and is based on cucumber [gherkin3 parser][gherkin].

**Godog** does not intervene with the standard **go test** command and it's behavior. You can leverage both frameworks
to functionally test your application while maintaining all test related source code in **_test.go** files.

**Godog** acts similar compared to **go test** command. It builds all package sources to a single main package file
and replaces **main** func with it's own and runs the build to test described application behavior in feature files.
Production builds remain clean without any test related source code.

### Install

    go get github.com/DATA-DOG/godog/cmd/godog

### Example

The following example can be [found here](https://github.com/DATA-DOG/godog/tree/master/examples/godogs).

#### Step 1

Imagine we have a **godog cart** to serve godogs for dinner. At first, we describe our feature
in plain text:

``` gherkin
# file: examples/godogs/godog.feature
Feature: eat godogs
  In order to be happy
  As a hungry gopher
  I need to be able to eat godogs

  Scenario: Eat 5 out of 12
    Given there are 12 godogs
    When I eat 5
    Then there should be 7 remaining
```

As a developer, your work is done as soon as you’ve made the program behave as
described in the Scenario.

#### Step 2

If you run `godog godog.feature` inside the **examples/godogs** directory.
You should see that the steps are undefined:

![Screenshot](https://raw.github.com/DATA-DOG/godog/master/screenshots/undefined.png)

It gives you undefined step snippets to implement in your test context. You may copy these snippets
into your `*_test.go` file.

Now if you run the tests again. You should see that the definition is now pending. You may change
**ErrPending** to **nil** and the scenario will pass successfully.

Since we need a working implementation, we may start by implementing only what is necessary.

#### Step 3

We only need a number of **godogs** for now. Lets define steps.

``` go
/* file: examples/godogs/godog.go */
package main

// Godogs to eat
var Godogs int

func main() { /* usual main func */ }
```

#### Step 4

Now lets finish our step implementations in order to test our feature requirements:

``` go
/* file: examples/godogs/godog_test.go */
package main

import (
	"fmt"

	"github.com/DATA-DOG/godog"
)

func thereAreGodogs(available int) error {
	Godogs = available
	return nil
}

func iEat(num int) error {
	if Godogs < num {
		return fmt.Errorf("you cannot eat %d godogs, there are %d available", num, Godogs)
	}
	Godogs -= num
	return nil
}

func thereShouldBeRemaining(remaining int) error {
	if Godogs != remaining {
		return fmt.Errorf("expected %d godogs to be remaining, but there is %d", remaining, Godogs)
	}
	return nil
}

func featureContext(s *godog.Suite) {
	s.Step(`^there are (\d+) godogs$`, thereAreGodogs)
	s.Step(`^I eat (\d+)$`, iEat)
	s.Step(`^there should be (\d+) remaining$`, thereShouldBeRemaining)

	s.BeforeScenario(func(interface{}) {
		Godogs = 0 // clean the state before every scenario
	})
}
```

Now when you run the `godog godog.feature` again, you should see:

![Screenshot](https://raw.github.com/DATA-DOG/godog/master/screenshots/passed.png)

**Note:** we have hooked to **BeforeScenario** event in order to reset state. You may hook into
more events, like **AfterStep** to test against an error and print more details about the error
or state before failure. Or **BeforeSuite** to prepare a database.

### Documentation

See [godoc][godoc] for general API details.
See **.travis.yml** for supported **go** versions.
See `godog -h` for general command options.

See implementation examples:

- [rest API server](https://github.com/DATA-DOG/godog/tree/master/examples/api)
- [godogs](https://github.com/DATA-DOG/godog/tree/master/examples/godogs)

### Changes

**2015-07-03**
- changed **godog.Suite** from interface to struct. Context registration should be updated accordingly. The reason
for change: since it exports the same methods and there is no need to mock a function in tests, there is no
obvious reason to keep an interface.
- in order to support running suite concurrently, needed to refactor an entry point of application. The **Run** method
now is a func of godog package which initializes and run the suite (or more suites). Method **New** is removed. This
change made godog a little cleaner.
- renamed **RegisterFormatter** func to **Format** to be more consistent.

### FAQ

**Q:** Where can I configure common options globally?
**A:** You can't. Alias your common or project based commands: `alias godog-wip="godog --format=progress --tags=@wip"`

### Contributions

Feel free to open a pull request. Note, if you wish to contribute an extension to public (exported methods or types) -
please open an issue before to discuss whether these changes can be accepted. All backward incompatible changes are
and will be treated cautiously.

### License

All package dependencies are **MIT** or **BSD** licensed.

**Godog** is licensed under the [three clause BSD license][license]

[godoc]: http://godoc.org/github.com/DATA-DOG/godog "Documentation on godoc"
[golang]: https://golang.org/  "GO programming language"
[behat]: http://docs.behat.org/ "Behavior driven development framework for PHP"
[cucumber]: https://cucumber.io/ "Behavior driven development framework for Ruby"
[gherkin]: https://github.com/cucumber/gherkin-go "Gherkin3 parser for GO"
[license]: http://en.wikipedia.org/wiki/BSD_licenses "The three clause BSD license"
