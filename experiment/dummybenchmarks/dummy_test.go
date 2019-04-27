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

package dummybenchmarks

import "testing"

// TestDontRun is for validating that we are not running tests with benchmarks.
func TestDontRun(t *testing.T) {
	t.Log("This is a Test not a Benchmark!")
}

func BenchmarkSimple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DoTheThing()
	}
}

func Benchmark(b *testing.B) {
	BenchmarkSimple(b)
}

func BenchmarkReportAllocs(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		DoTheThing()
	}
}

func BenchmarkSetBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DoTheThing()
		b.SetBytes(20)
	}
}

func BenchmarkAllocsAndBytes(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		DoTheThing()
		b.SetBytes(20)
	}
}

func BenchmarkParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			DoTheThing()
		}
	})
}

func BenchmarkLog(b *testing.B) {
	b.Logf("About to DoTheThing() x%d.", b.N)
	for i := 0; i < b.N; i++ {
		DoTheThing()
	}
}

func BenchmarkSkip(b *testing.B) {
	b.Skip("This Benchmark is skipped.")
}

func BenchmarkSkipNow(b *testing.B) {
	b.SkipNow()
}

func BenchmarkError(b *testing.B) {
	b.Error("Early Benchmark error.")
	BenchmarkLog(b)
}

func BenchmarkFatal(b *testing.B) {
	b.Fatal("This Benchmark failed.")
}

func BenchmarkFailNow(b *testing.B) {
	b.FailNow()
}

func BenchmarkNestedShallow(b *testing.B) {
	b.Run("simple", BenchmarkSimple)
	b.Run("parallel", BenchmarkParallel)
}

func BenchmarkNestedDeep(b *testing.B) {
	b.Run("1", func(b1 *testing.B) {
		b.Run("1 simple", BenchmarkSimple)
		b.Run("1 parallel", BenchmarkParallel)

		b.Run("2", func(b2 *testing.B) {
			b.Run("3A", func(b3 *testing.B) {
				b.Run("3A simple", BenchmarkSimple)
			})
			b.Run("3B", func(b3 *testing.B) {
				b.Run("3B parallel", BenchmarkParallel)
			})
		})
	})
}
