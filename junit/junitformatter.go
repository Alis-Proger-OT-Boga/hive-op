package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"

	"github.com/ethereum/hive/internal/libhive"
)

func main() {
	if len(os.Args) <= 1 {
		fail(errors.New("no input files specified"))
	}

	result := TestSuites{
		Failures: 0,
		Name:     "Hive Results",
		Tests:    0,
	}
	var suites []TestSuite

	for i := 1; i < len(os.Args); i++ {
		inData, err := os.ReadFile(os.Args[i])

		if err != nil {
			fail(err)
		}
		var suite libhive.TestSuite
		err = json.Unmarshal(inData, &suite)
		if err != nil {
			fail(err)
		}
		junitSuite := TestSuite{
			Name:       suite.Name,
			Failures:   0,
			Tests:      len(suite.TestCases),
			Properties: Properties{},
		}
		for clientName, clientVersion := range suite.ClientVersions {
			junitSuite.Properties.Properties = append(junitSuite.Properties.Properties, Property{
				Name:  clientName,
				Value: clientVersion,
			})
		}
		for _, testCase := range suite.TestCases {
			if !testCase.SummaryResult.Pass {
				junitSuite.Failures = junitSuite.Failures + 1
			}
			junitCase := TestCase{
				Name: testCase.Name,
			}
			if testCase.SummaryResult.Pass {
				junitCase.SystemOut = testCase.SummaryResult.Details
			} else {
				junitCase.Failure = &Failure{Message: testCase.SummaryResult.Details}
			}
			junitSuite.TestCases = append(junitSuite.TestCases, junitCase)
		}
		result.Failures = result.Failures + junitSuite.Failures
		result.Tests = result.Tests + junitSuite.Tests
		suites = append(suites, junitSuite)
	}
	result.Suites = suites

	junit, err := xml.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	fmt.Println(string(junit))
}

func fail(reason error) {
	fmt.Println(reason)
	os.Exit(1)
}

/*
Target XML format (lots of it being optional):
<testsuites disabled="" errors="" failures="" name="" tests="" time="">
    <testsuite disabled="" errors="" failures="" hostname="" id=""
               name="" package="" skipped="" tests="" time="" timestamp="">
        <properties>
            <property name="" value=""/>
        </properties>
        <testcase assertions="" classname="" name="" status="" time="">
            <skipped/>
            <error message="" type=""/>
            <failure message="" type=""/>
            <system-out/>
            <system-err/>
        </testcase>
        <system-out/>
        <system-err/>
    </testsuite>
</testsuites>
*/

type TestSuites struct {
	XMLName  string      `xml:"testsuites,omitempty"`
	Failures int         `xml:"failures,attr"`
	Name     string      `xml:"name,attr"`
	Tests    int         `xml:"tests,attr"`
	Suites   []TestSuite `xml:"testsuite"`
}

type TestSuite struct {
	Name       string     `xml:"name,attr"`
	Failures   int        `xml:"failures,attr"`
	Tests      int        `xml:"tests,attr"`
	Properties Properties `xml:"properties,omitempty"`
	TestCases  []TestCase `xml:"testcase"`
}

type TestCase struct {
	Name      string   `xml:"name,attr"`
	Failure   *Failure `xml:"failure,omitempty"`
	SystemOut string   `xml:"system-out,omitempty"`
}

type Failure struct {
	Message string `xml:"message,attr"`
}

type Properties struct {
	Properties []Property `xml:"property,omitempty"`
}

type Property struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}
