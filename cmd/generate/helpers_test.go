package generate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func getAbsPath(t *testing.T) string {
	t.Helper()

	absPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		t.Fatal(err)
	}

	return absPath
}

func jsonPrettyPrint(t *testing.T, i interface{}) string {
	t.Helper()

	byt, err := json.MarshalIndent(i, "", "\t")
	if err != nil {
		t.Fatal(err)
	}

	return string(byt)
}
