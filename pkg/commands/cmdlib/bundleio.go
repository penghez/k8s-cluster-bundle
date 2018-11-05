// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmdlib

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/golang/glog"

	"github.com/GoogleCloudPlatform/k8s-cluster-bundle/pkg/converter"
	"github.com/GoogleCloudPlatform/k8s-cluster-bundle/pkg/core"
	"github.com/GoogleCloudPlatform/k8s-cluster-bundle/pkg/files"
	"github.com/GoogleCloudPlatform/k8s-cluster-bundle/pkg/inline"
)

// ReadComponentData reads component data file contents from a file or stdin.
func ReadComponentData(ctx context.Context, rw files.FileReaderWriter, g *GlobalOptions) (*core.ComponentData, error) {
	var bytes []byte
	var err error
	if g.ComponentDataFile != "" {
		log.Infof("Reading component data file %v", g.ComponentDataFile)
		bytes, err = rw.ReadFile(ctx, g.ComponentDataFile)
		if err != nil {
			return nil, err
		}
	} else {
		log.Info("No component data file, reading from stdin")
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			bytes = append(bytes, scanner.Bytes()...)
			bytes = append(bytes, '\n')
		}
		if err := scanner.Err(); err != nil {
			return nil, err
		}
	}

	b, err := convertComponentData(ctx, bytes, g)
	if err != nil {
		return nil, err
	}

	// For now, we can only inline component data files because we need the path
	// context.
	if (g.InlineComponents || g.InlineObjects) && g.ComponentDataFile != "" {
		return inlineData(ctx, b, rw, g)
	}
	return b, nil
}

func convertComponentData(ctx context.Context, bt []byte, g *GlobalOptions) (*core.ComponentData, error) {
	return converter.FromContentType(g.InputFormat, bt).ToComponentData()
}

// inlineData inlines a cluster bundle before processing
func inlineData(ctx context.Context, data *core.ComponentData, rw files.FileReaderWriter, g *GlobalOptions) (*core.ComponentData, error) {
	inliner := &inline.Inliner{
		&files.LocalFileObjReader{
			WorkingDir: filepath.Dir(g.ComponentDataFile),
			Rdr:        rw,
		},
	}
	var err error
	if g.InlineComponents {
		data, err = inliner.InlineComponentDataFiles(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("error inlining component data files: %v", err)
		}
	}
	if g.InlineObjects {
		data, err = inliner.InlineComponentsInData(ctx, data)
		if err != nil {
			return nil, fmt.Errorf("error inlining objects: %v", err)
		}
	}
	return data, nil
}

// WriteContentsStructured writes some structured contents. The contents must be serializable to both JSON and YAML.
func WriteStructuredContents(ctx context.Context, obj interface{}, rw files.FileReaderWriter, g *GlobalOptions) error {
	bytes, err := converter.FromObject(obj).ToContentType(g.OutputFormat)
	if err != nil {
		return fmt.Errorf("error writing contents: %v", err)
	}
	return WriteContents(ctx, g.OutputFile, bytes, rw)
}

// WriteContents writes some bytes to disk or stdout. if outPath is empty, write to stdout instdea.
func WriteContents(ctx context.Context, outPath string, bytes []byte, rw files.FileReaderWriter) error {
	if outPath == "" {
		_, err := os.Stdout.Write(bytes)
		if err != nil {
			return fmt.Errorf("error writing content %q to stdout", string(bytes))
		}
		return nil
	}
	log.Infof("Writing file to %q", outPath)
	return rw.WriteFile(ctx, outPath, bytes, DefaultFilePermissions)
}
