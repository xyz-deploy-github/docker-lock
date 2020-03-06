package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/michaelperel/docker-lock/cmd"
)

func main() {
	if os.Args[1] == "docker-cli-plugin-metadata" {
		m := map[string]string{
			"SchemaVersion":    "0.1.0",
			"Vendor":           "https://github.com/michaelperel/docker-lock",
			"Version":          cmd.Version,
			"ShortDescription": "Manage Lockfiles",
		}
		j, _ := json.Marshal(m)
		fmt.Println(string(j))
		os.Exit(0)
	}
	if err := cmd.Execute(nil); err != nil {
		fmt.Fprint(os.Stderr, err)
		fmt.Println()
		os.Exit(1)
	}
}
