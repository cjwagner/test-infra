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
	"io/ioutil"
	"testing"

	"k8s.io/test-infra/testgrid/metadata/junit"
)

func createTempFile() (string, error) {
	outFile, err := ioutil.TempFile("", "dummybenchmarks")
	if err != nil {
		return "", fmt.Errorf("create error: %v", err)
	}
	if err := outFile.Close(); err != nil {
		return "", fmt.Errorf("close error: %v", err)
	}
	return outFile.Name(), nil
}

func TestDummybenchmarksIntegration(t *testing.T) {
	outFile, err := createTempFile()
	if err != nil {
		t.Fatalf("Error creating output file: %v.", err)
	}
	//defer os.Remove(outFile)
	logFile, err := createTempFile()
	if err != nil {
		t.Fatalf("Error creating log file: %v.", err)
	}
	//defer os.Remove(logFile)
	t.Logf("Logging benchmark output to %q.", logFile)
	opts := &options{
		outputFile: outFile,
	}

	t.Logf("Starting benchmarkjunit outputting to %q...", opts.outputFile)
	run(opts, []string{"../../experiment/dummybenchmarks/..."})
	t.Log("Finished running benchmarkjunit. Validating JUnit XML...")

	raw, err := ioutil.ReadFile(opts.outputFile)
	if err != nil {
		t.Fatalf("Error reading output file: %v.", err)
	}

	suites, err := junit.Parse(raw)
	if err != nil {
		t.Fatalf("Error parsing JUnit XML testsuites: %v.", err)
	}
	if len(suites.Suites) != 2 {
		t.Fatalf("Expected 2 testsuites, but found %d.", len(suites.Suites))
	}
}
