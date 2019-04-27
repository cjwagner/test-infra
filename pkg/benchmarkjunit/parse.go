/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	// reSuiteStart identifies the start of a new TestSuite and captures the package path.
	// Matches lines like "pkg: k8s.io/test-infra/experiment/dummybenchmarks"
	reSuiteStart = regexp.MustCompile(`^pkg:\s+(\S+)\s*$`)
	// reSuiteEnd identifies the end of a TestSuite and captures the overall result, package path, and runtime.
	// Matches lines like:
	// "ok  	k8s.io/test-infra/experiment/dummybenchmarks/subpkg	1.490s"
	// "FAIL	k8s.io/test-infra/experiment/dummybenchmarks	17.829s"
	reSuiteEnd = regexp.MustCompile(`^(ok|FAIL)\s+(\S+)\s+(\S+)\s*$`)
	// reBenchMetrics identifies lines with metrics for successful Benchmarks and captures the name, op count, and metric values.
	// Matches lines like:
	// "Benchmark-4                 	20000000	       77.9 ns/op"
	// "BenchmarkAllocsAndBytes-4   	10000000	       131 ns/op	 152.50 MB/s	     112 B/op	       2 allocs/op"
	reBenchMetrics = regexp.MustCompile(`^(Benchmark\S*)\s+(\d+)\s+([\d\.]+) ns/op(?:\s+([\d\.]+) MB/s)?(?:\s+([\d\.]+) B/op)?(?:\s+([\d\.]+) allocs/op)?\s*$`)
	// reActionLine identifies lines that start with "--- " and denote the start of log output and/or a skipped or failed Benchmark.
	// Matches lines like:
	// "--- BENCH: BenchmarkLog-4"
	// "--- SKIP: BenchmarkSkip"
	// "--- FAIL: BenchmarkFatal"
	reActionLine = regexp.MustCompile(`^--- (BENCH|SKIP|FAIL):\s+(\S+)\s*$`)
)

// Property defines the xml element that stores additional metrics about each benchmark.
type Property struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// Properties defines the xml element that stores the list of properties that are associated with one benchmark.
type Properties struct {
	PropertyList []Property `xml:"property"`
}

// Failure defines the xml element that stores failure text.
type Failure struct {
	Text string `xml:",chardata"`
}

// TestCase defines the xml element that stores all information associated with one benchmark.
type TestCase struct {
	ClassName    string     `xml:"class_name,attr"`
	Name         string     `xml:"name,attr"`
	Time         string     `xml:"time,attr"`
	Failure      *Failure   `xml:"failure,omitempty"`
	Skipped      bool       `xml:"skipped,omitempty"`
	PropertyList Properties `xml:"properties"`
}

// TestSuite defines the outer-most xml element that contains all TestCases.
type TestSuite struct {
	Name      string     `xml:"name,attr"`
	Tests     int        `xml:"tests,attr"`
	Failures  int        `xml:"failures,attr"`
	Time      string     `xml:"time,attr"`
	TestCases []TestCase `xml:"testcase"`
}

// TestSuites defines the xml element that stores the list of TestSuites
type TestSuites struct {
	XMLName string      `xml:"testsuites"`
	List    []TestSuite `xml:"testsuite"`
}

func truncate(str string, n int) string {
	if len(str) <= n {
		return str
	}
	if n > 3 {
		return str[:n-3] + "..."
	}
	return str[:n]
}

func (s *TestSuite) recordLogText(text string) {
	if len(s.TestCases) == 0 {
		logrus.Error("Tried to record Benchmark log text before any Benchmarks were found for the package!")
		return
	}
	tc := &s.TestCases[len(s.TestCases)-1]
	// TODO: make failure text truncation configurable.
	text = truncate(text, 1000)
	// For now only record log text for failures.
	if tc.Failure == nil {
		return
	}
	tc.Failure.Text = text
	// Also add failure text to "categorized_fail" property for TestGrid.
	tc.PropertyList.PropertyList = append(
		tc.PropertyList.PropertyList,
		Property{Name: "categorized_fail", Value: text},
	)
}

// propertiesFromMatch returns the PropertyList and test duration for a Benchmark metric line match (reBenchMetrics).
func propertiesFromMatch(match []string) (*Properties, string, error) {
	var props []Property
	opCount, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		return nil, "", fmt.Errorf("error parsing opcount %q: %v", match[2], err)
	}
	opDuration, err := strconv.ParseFloat(match[3], 64)
	if err != nil {
		return nil, "", fmt.Errorf("error parsing ns/op %q: %v", match[3], err)
	}
	benchmarkDuration := fmt.Sprintf("%f", opCount*opDuration/1000000000) // convert from ns to s.

	props = append(props, Property{Name: "op count", Value: match[2]})
	props = append(props, Property{Name: "avg op duration (ns/op)", Value: match[3]})
	if len(match[4]) > 0 {
		props = append(props, Property{Name: "MB/s", Value: match[4]})
	}
	if len(match[5]) > 0 {
		props = append(props, Property{Name: "alloced B/op", Value: match[5]})
	}
	if len(match[6]) > 0 {
		props = append(props, Property{Name: "allocs/op", Value: match[6]})
	}

	return &Properties{PropertyList: props}, benchmarkDuration, nil
}

func parse(raw []byte) (*TestSuites, error) {
	lines := strings.Split(string(raw), "\n")

	var suites TestSuites
	var suite TestSuite
	var logText string
	for _, line := range lines {
		// First handle multi-line log text aggregation
		if strings.HasPrefix(line, "    ") {
			logText += strings.TrimPrefix(line, "    ") + "\n"
			continue
		} else if len(logText) > 0 {
			suite.recordLogText(logText)
			logText = ""
		}

		switch {
		case reSuiteStart.MatchString(line):
			match := reSuiteStart.FindStringSubmatch(line)
			suite = TestSuite{
				Name: match[1],
			}

		case reSuiteEnd.MatchString(line):
			match := reSuiteEnd.FindStringSubmatch(line)
			if match[2] != suite.Name {
				return nil, fmt.Errorf("mismatched package summary for %q with %q benchmarks", match[2], suite.Name)
			}
			duration, err := time.ParseDuration(match[3])
			if err != nil {
				return nil, fmt.Errorf("failed to parse package test time %q: %v", match[3], err)
			}
			suite.Time = fmt.Sprintf("%f", duration.Seconds())
			suites.List = append(suites.List, suite)

		case reBenchMetrics.MatchString(line):
			match := reBenchMetrics.FindStringSubmatch(line)
			tc := TestCase{
				ClassName: path.Base(suite.Name),
				Name:      match[1],
			}
			props, duration, err := propertiesFromMatch(match)
			if err != nil {
				return nil, fmt.Errorf("error parsing benchmark metric values: %v", err)
			}
			tc.PropertyList, tc.Time = *props, duration
			suite.TestCases = append(suite.TestCases, tc)
			suite.Tests += 1

		case reActionLine.MatchString(line):
			match := reActionLine.FindStringSubmatch(line)
			if match[1] == "SKIP" {
				suite.TestCases = append(suite.TestCases, TestCase{
					ClassName: path.Base(suite.Name),
					Name:      match[2],
					Time:      "0",
					Skipped:   true,
				})
			} else if match[1] == "FAIL" {
				suite.TestCases = append(suite.TestCases, TestCase{
					ClassName: path.Base(suite.Name),
					Name:      match[2],
					Time:      "0",
					Failure:   &Failure{},
				})
				suite.Failures += 1
				suite.Tests += 1
			}
		}
	}
	return &suites, nil
}
