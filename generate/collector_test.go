package generate_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/safe-waters/docker-lock/generate"
)

func TestNewCollector(t *testing.T) {
	t.Parallel()

	_, err := generate.NewCollector("", nil, nil, nil, true)
	if err == nil {
		t.Fatal(err)
	}
}

func TestCollector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name                  string
		Collector             *generate.Collector
		AddTempDirToCollector bool
		BaseDirIsTempDir      bool
		ShouldFail            bool
		Expected              []string
		PathsToCreate         []string
	}{
		{
			Name: "Default Path Exists",
			Collector: &generate.Collector{
				DefaultPaths: []string{"Dockerfile"},
			},
			AddTempDirToCollector: true,
			PathsToCreate:         []string{"Dockerfile"},
			Expected:              []string{"Dockerfile"},
		},
		{
			Name: "Default Path Does Not Exist",
			Collector: &generate.Collector{
				DefaultPaths: []string{"Dockerfile"},
			},
		},
		{
			Name: "Do Not Use Default Paths If Other Methods Specified",
			Collector: &generate.Collector{
				DefaultPaths: []string{"Dockerfile"},
				ManualPaths:  []string{"Dockerfile-Manual"},
			},
			AddTempDirToCollector: true,
			Expected:              []string{"Dockerfile-Manual"},
			PathsToCreate:         []string{"Dockerfile-Manual"},
		},
		{
			Name: "Manual Paths",
			Collector: &generate.Collector{
				ManualPaths: []string{"Dockerfile"},
			},
			AddTempDirToCollector: true,
			Expected:              []string{"Dockerfile"},
			PathsToCreate:         []string{"Dockerfile"},
		},
		{
			Name: "Globs",
			Collector: &generate.Collector{
				Globs: []string{filepath.Join("**", "Dockerfile")},
			},
			AddTempDirToCollector: true,
			Expected: []string{
				filepath.Join("globs-test", "Dockerfile"),
			},
			PathsToCreate: []string{
				filepath.Join("globs-test", "Dockerfile"),
			},
		},
		{
			Name: "Recursive",
			Collector: &generate.Collector{
				DefaultPaths: []string{"Dockerfile"},
				Recursive:    true,
			},
			BaseDirIsTempDir: true,
			Expected: []string{
				filepath.Join("recursive-test", "Dockerfile"),
			},
			PathsToCreate: []string{
				filepath.Join("recursive-test", "Dockerfile"),
			},
		},
		{
			Name: "Path Outside Of Base Directory",
			Collector: &generate.Collector{
				ManualPaths: []string{filepath.Join("..", "Dockerfile")},
			},
			ShouldFail: true,
		},
		{
			Name: "Duplicate Paths",
			Collector: &generate.Collector{
				ManualPaths: []string{"Dockerfile", "Dockerfile"},
			},
			AddTempDirToCollector: true,
			Expected:              []string{"Dockerfile"},
			PathsToCreate:         []string{"Dockerfile"},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.Name, func(t *testing.T) {
			t.Parallel()

			var expected []string

			if len(test.PathsToCreate) != 0 {
				tempDir := makeTempDir(t, dockerfileParserTestDir)
				defer os.RemoveAll(tempDir)

				if test.BaseDirIsTempDir {
					test.Collector.BaseDir = tempDir
				}

				if test.AddTempDirToCollector {
					addTempDirToStringSlices(t, test.Collector, tempDir)
				}

				makeParentDirsInTempDirFromFilePaths(
					t, tempDir, test.PathsToCreate,
				)
				pathsToCreateContents := make([][]byte, len(test.PathsToCreate))
				writeFilesToTempDir(
					t, tempDir, test.PathsToCreate, pathsToCreateContents,
				)

				for _, path := range test.Expected {
					expected = append(
						expected, filepath.Join(tempDir, path),
					)
				}
			}

			var got []string

			var err error

			done := make(chan struct{})
			for pathResult := range test.Collector.Paths(done) {
				if pathResult.Err != nil {
					close(done)
					err = pathResult.Err
					break
				}
				got = append(got, pathResult.Path)
			}

			if test.ShouldFail {
				if err == nil {
					t.Fatal("expected an error but did not get one")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			assertCollectedPathsEqual(t, expected, got)
		})
	}
}

func assertCollectedPathsEqual(t *testing.T, expected []string, got []string) {
	t.Helper()

	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func addTempDirToStringSlices(
	t *testing.T,
	collector *generate.Collector,
	tempDir string,
) {
	t.Helper()

	collectorValue := reflect.ValueOf(collector).Elem()

	for i := 0; i < collectorValue.NumField(); i++ {
		field := collectorValue.Field(i)

		if field.Kind() == reflect.Slice {
			concreteSliceField := field.Interface().([]string)

			for i := 0; i < len(concreteSliceField); i++ {
				concreteSliceField[i] = filepath.Join(
					tempDir, concreteSliceField[i],
				)
			}
		}
	}
}
