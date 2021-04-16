package main

import (
	"fmt"
	"log"

	"github.com/robertkrimen/otto"
	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	"github.com/sensu/sensu-go/types"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	Example string
}

var (
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-metric-analysis",
			Short:    "Check that lets you evaluate JSON returned from REST API endpoints using Javascript conditional expressions",
			Keyspace: "sensu.io/plugins/sensu-metric-analysis/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		&sensu.PluginConfigOption{
			Path:      "example",
			Env:       "CHECK_EXAMPLE",
			Argument:  "example",
			Shorthand: "e",
			Default:   "",
			Usage:     "An example string configuration option",
			Value:     &plugin.Example,
		},
	}
)

func main() {
	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *types.Event) (int, error) {
	if len(plugin.Example) == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--example or CHECK_EXAMPLE environment variable is required")
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(event *types.Event) (int, error) {
	log.Println("executing check with --example", plugin.Example)
	return sensu.CheckStateOK, nil
}

func processResult(data string, jscript string) {
	fmt.Println("Hello, playground")
	vm := otto.New()

	vm.Set("data", data)

	return_value, err := vm.Run(jscript)
	if err != nil {
		fmt.Printf("vm.Run error: %v", err)
	} else {
		if return_bool, err := return_value.ToBoolean(); err == nil {
			if err != nil {
				fmt.Printf("return_value.ToBoolean error: %v", err)
			} else {
				fmt.Printf("Return_bool: %v", return_bool)
			}
		}
	}
}
