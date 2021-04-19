package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/robertkrimen/otto"
	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	"github.com/sensu/sensu-go/types"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	//low level arguments for any http request:
	Timeout int
	Headers []string
	Request string
	Url     string
	Eval    string
	Query   string
	Type    string
}

var (
	tlsConfig tls.Config
	plugin    = Config{
		PluginConfig: sensu.PluginConfig{
			Name:     "sensu-metric-analysis",
			Short:    "Check that lets you evaluate JSON returned from REST API endpoints using Javascript conditional expressions",
			Keyspace: "sensu.io/plugins/sensu-metric-analysis/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		&sensu.PluginConfigOption{
			Path:      "timeout",
			Env:       "",
			Argument:  "timeout",
			Shorthand: "T",
			Default:   15,
			Usage:     "Request timeout in seconds",
			Value:     &plugin.Timeout,
		},
		&sensu.PluginConfigOption{
			Path:      "url",
			Env:       "",
			Argument:  "url",
			Shorthand: "U",
			Default:   "",
			Usage:     "url to use ex: https://httpbin.org/post",
			Value:     &plugin.Url,
		},
		&sensu.PluginConfigOption{
			Path:      "request",
			Env:       "",
			Argument:  "request",
			Shorthand: "r",
			Default:   "GET",
			Usage:     "Optional. Default to get, if --query is used changes to post",
			Value:     &plugin.Request,
		},
		{
			Path:      "header",
			Env:       "",
			Argument:  "header",
			Shorthand: "H",
			Default:   []string{},
			Usage:     "Additional header(s) to send in check request",
			Value:     &plugin.Headers,
		},
		&sensu.PluginConfigOption{
			Path:      "eval",
			Env:       "",
			Argument:  "eval",
			Shorthand: "e",
			Default:   "",
			Usage:     `Javascript to evaluate, must return javascript boolean  Ex: data.test === "value"`,
			Value:     &plugin.Eval,
		},
		&sensu.PluginConfigOption{
			Path:      "query",
			Env:       "",
			Argument:  "query",
			Shorthand: "q",
			Default:   "",
			Usage:     `query data`,
			Value:     &plugin.Query,
		},
		&sensu.PluginConfigOption{
			Path:      "type",
			Env:       "",
			Argument:  "type",
			Shorthand: "t",
			Default:   "",
			Usage:     `Optional (no default is set). Sets --request, --headers, --port, --path, and --params based on the backend type (e.g. prometheus, elasticsearch, or influxdb). Setting --type=prometheus`,
			Value:     &plugin.Query,
		},
	}
)

func main() {
	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func checkArgs(event *types.Event) (int, error) {
	if len(plugin.Url) == 0 {
		return sensu.CheckStateWarning, fmt.Errorf("--url is required")
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(event *types.Event) (int, error) {
	log.Printf("Request Method: %v\n", plugin.Request)
	log.Printf("Url: %v\n", plugin.Url)
	log.Printf("Headers: %v\n", plugin.Headers)
	log.Printf("Query: %v\n", plugin.Query)
	log.Printf("Eval: %v\n", plugin.Eval)
	response, err := doQuery(plugin.Url, plugin.Request, strings.NewReader(plugin.Query))
	log.Printf("http response: %v\n", string(response))
	if err != nil {
		log.Printf("Error attempting query http request: %v", err)
		return sensu.CheckStateCritical, err
	}
	result, err := processResponse(string(response), plugin.Eval)
	log.Printf("eval result: %v\n", result)
	if err != nil {
		log.Printf("Error attempting to evaluate http response: %v", err)
		return sensu.CheckStateCritical, err
	}
	if result != true {
		return sensu.CheckStateWarning, nil
	}
	return sensu.CheckStateOK, nil
}

func doQuery(urlString string, requestType string, data io.Reader) ([]byte, error) {
	client := http.DefaultClient
	client.Transport = http.DefaultTransport
	client.Timeout = time.Duration(plugin.Timeout) * time.Second
	checkURL, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}
	if checkURL.Scheme == "https" {
		client.Transport.(*http.Transport).TLSClientConfig = &tlsConfig
	}

	req, err := http.NewRequest(requestType, urlString, data)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	if len(plugin.Headers) > 0 {
		for _, header := range plugin.Headers {
			headerSplit := strings.SplitN(header, ":", 2)
			req.Header.Set(strings.TrimSpace(headerSplit[0]), strings.TrimSpace(headerSplit[1]))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var jsonBody interface{}

	err = json.Unmarshal(body, &jsonBody)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshal response body into JSON: %v", err)
	}
	return body, nil
}

func processResponse(data string, jscript string) (bool, error) {
	vm := otto.New()

	vm.Set("input", data)
	vm.Run(`
          data = JSON.parse(input)
        `)
	return_value, err := vm.Run(jscript)
	if err != nil {
		log.Printf("vm.Run error: %v", err)
		return false, err
	} else {
		if return_bool, err := return_value.ToBoolean(); err == nil {
			if err != nil {
				log.Printf("return_value.ToBoolean error: %v\n", err)
				return return_bool, err
			} else {
				return return_bool, err
			}
		}
	}
	return false, fmt.Errorf("processResponse: unknown error")
}
