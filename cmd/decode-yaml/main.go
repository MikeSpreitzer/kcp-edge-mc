/*
Copyright 2024 The KubeStellar Authors.

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
	"io"
	"os"

	"github.com/kubestellar/kubestellar/pkg/crd"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "%s Usage: $file_pathname\n", os.Args[0])
		os.Exit(1)
	}
	inputPath := os.Args[1]
	file, err := os.Open(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open %s for reading: %s", inputPath, err)
		os.Exit(10)
	}
	inputBytes, err := io.ReadAll(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read %s: %s", inputPath, err)
		os.Exit(20)
	}

	_, err = crd.DecodeYAML(inputBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decode %s: %s", inputPath, err)
		os.Exit(30)
	}

}
