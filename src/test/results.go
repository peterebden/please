// Code for parsing the output of tests.

package test

import (
	"fmt"
	"io/ioutil"

	"core"
)

// parseTestResults converts the raw test result data to test results.
func parseTestResults(target *core.BuildTarget, data [][]byte) (*core.TestResults, error) {
	for _, d := range data {
		r, err := parseSingleResult(d)
		if err != nil {
			return nil, err
		}
		target.Results.Aggregate(r)
	}
	// Ensure that there is one success if the target succeeded but there are no tests.
	if target.Results.Failed == 0 && target.Results.NumTests == 0 {
		target.Results.NumTests++
		target.Results.Passed++
	}
	return &target.Results, nil
}

// parseTestResultsFile reads a file and converts the contents to test results.
func parseTestResultsFile(target *core.BuildTarget, filename string) (*core.TestResults, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return parseTestResults(target, [][]byte{data})
}

func parseSingleResult(data []byte) (*core.TestResults, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("No results")
	} else if looksLikeGoTestResults(data) {
		return parseGoTestResults(data)
	} else {
		return parseJUnitXMLTestResults(data)
	}
}
