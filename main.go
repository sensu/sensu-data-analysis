package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
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
	Timeout        int
	Headers        []string
	Request        string
	Url            string
	EvalStatements []string
	EvalStatus     int
	Query          string
	Type           string
	Verbose        bool
	DryRun         bool
	Scheme         string
	Host           string
	Port           int
	ApiPath        string
	ApiParams      string
}

type ServiceType struct {
	Scheme    string
	Host      string
	Port      int
	ApiPath   string
	ApiParams string
	Request   string
	Headers   []string
}

var (
	tlsConfig tls.Config
	//Map of supported services and their default request values
	supportedServices = map[string]ServiceType{
		"prometheus": ServiceType{
			Scheme:    "http",
			Host:      "localhost",
			Port:      9090,
			ApiPath:   "api/v1/query",
			ApiParams: "query=up",
			Request:   "POST",
			Headers: []string{
				"Content-Type: application/x-www-form-urlencoded",
			},
		},
	}
	//
	plugin = Config{
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
			Path:     "eval-status",
			Env:      "",
			Argument: "eval-status",
			Default:  1,
			Usage:    "Return status if any eval statement condition is not met (eg. a metric exceeds a threshold). Must be >= 1.",
			Value:    &plugin.EvalStatus,
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
			Default:   "",
			Usage:     "Optional. Default to get, if --query is used changes to post",
			Value:     &plugin.Request,
		},
		{
			Path:      "headers",
			Env:       "",
			Argument:  "header",
			Shorthand: "H",
			Default:   []string{},
			Usage:     "Additional header(s) to send in check request",
			Value:     &plugin.Headers,
		},
		&sensu.PluginConfigOption{
			Path:      "eval-statements",
			Env:       "",
			Argument:  "eval-statement",
			Shorthand: "e",
			Default:   []string{},
			Usage: `Optional. Array of Javascript expressions that must return a bool. 
			The Javascript experssion is evaluated in a "sandbox" and is provided a single variable called 'result' that contains the complete query response in JSON format.  If no eval is required, the check will return the query response as output. Ex: result.test === "value"`,
			Value: &plugin.EvalStatements,
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
			Usage:     `Optional (no default is set). Sets --request, --header, --port, --path, and --params based on the backend type (e.g. prometheus, elasticsearch, or influxdb). Setting --type=prometheus`,
			Value:     &plugin.Type,
		},
		&sensu.PluginConfigOption{
			Argument:  "verbose",
			Shorthand: "v",
			Default:   false,
			Usage:     `Enable verbose output`,
			Value:     &plugin.Verbose,
		},
		&sensu.PluginConfigOption{
			Argument:  "dryrun",
			Shorthand: "n",
			Default:   false,
			Usage:     `Do not execute query, just report configuration. Useful for diagnostic testing`,
			Value:     &plugin.DryRun,
		},
		&sensu.PluginConfigOption{
			Path:     "scheme",
			Argument: "scheme",
			Usage:    `service scheme. http or https`,
			Value:    &plugin.Scheme,
		},
		&sensu.PluginConfigOption{
			Path:     "host",
			Argument: "host",
			Usage:    `service host`,
			Value:    &plugin.Host,
		},
		&sensu.PluginConfigOption{
			Path:     "port",
			Argument: "port",
			Usage:    `service port`,
			Value:    &plugin.Port,
		},
		&sensu.PluginConfigOption{
			Path:     "path",
			Argument: "path",
			Usage:    `service endpoint path`,
			Value:    &plugin.ApiPath,
		},
		&sensu.PluginConfigOption{
			Path:     "params",
			Argument: "params",
			Usage:    `request params`,
			Value:    &plugin.ApiParams,
		},
	}
)

func main() {
	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func serviceDefaults(service ServiceType) {
	if plugin.Verbose {
		log.Printf("Setting Service Defaults\n")
	}
	plugin.Headers = append(plugin.Headers, service.Headers...)
	if len(plugin.Request) == 0 {
		plugin.Request = service.Request
	}
	if len(plugin.Scheme) == 0 {
		plugin.Scheme = service.Scheme
	}
	if len(plugin.Host) == 0 {
		plugin.Host = service.Host
	}
	if plugin.Port == 0 {
		plugin.Port = service.Port
	}
	if len(plugin.ApiPath) == 0 {
		plugin.ApiPath = service.ApiPath
	}
	if len(plugin.ApiParams) == 0 {
		plugin.ApiParams = service.ApiParams
	}
}

func finalUrl() (string, error) {
	newUrl := plugin.Url
	if len(plugin.Type) > 0 {
		if service, found := supportedServices[plugin.Type]; found {
			if plugin.Verbose {
				log.Printf("Found Supported Service Type: %v\n", plugin.Type)
			}
			serviceDefaults(service)
		} else {
			if plugin.Verbose {
				log.Printf("Unknown Service Type: %v\n", plugin.Type)
			}
		}
		if len(newUrl) == 0 && len(plugin.Scheme) > 0 && len(plugin.Host) > 0 && plugin.Port > 0 {
			newUrl = fmt.Sprintf("%v://%v:%v/", plugin.Scheme, plugin.Host, plugin.Port)
			if len(plugin.ApiPath) > 0 {
				newUrl = fmt.Sprintf("%v%v", newUrl, plugin.ApiPath)
			}
			if len(plugin.ApiParams) > 0 {
				newUrl = fmt.Sprintf("%v?%v", newUrl, plugin.ApiParams)
			}
		}
	}
	if len(newUrl) == 0 {
		return newUrl, errors.New("final URL is empty")
	}
	_, err := url.Parse(newUrl)
	return newUrl, err

}

func checkArgs(event *types.Event) (int, error) {
	if plugin.DryRun {
		plugin.Verbose = true
	}
	newUrl, err := finalUrl()
	plugin.Url = newUrl

	if len(plugin.Request) == 0 {
		plugin.Request = `GET`
	}

	if plugin.Verbose {
		log.Printf("Type: %v\n", plugin.Type)
		log.Printf("Request Method: %v\n", plugin.Request)
		log.Printf("Url: %v\n", plugin.Url)
		log.Printf("Headers: %v\n", plugin.Headers)
		log.Printf("Query: %v\n", plugin.Query)
		log.Printf("Eval Statements: %v\n", plugin.EvalStatements)
		log.Printf("Supporterd Services:\n")
		for name, service := range supportedServices {
			log.Printf(" %v: %v\n", name, service)
		}
	}

	if err != nil {
		if plugin.DryRun {
			log.Printf("Warning: unexpected error associated with Url: %v", err)
		} else {
			return sensu.CheckStateWarning, err
		}
	}

	if plugin.EvalStatus < 1 {
		if plugin.DryRun {
			log.Printf("Warning: -eval-status >= 1 is required")
		} else {
			return sensu.CheckStateWarning, fmt.Errorf("--eval-status >= 1 is required")
		}
	}
	return sensu.CheckStateOK, nil
}

func executeCheck(event *types.Event) (int, error) {
	if plugin.DryRun {
		log.Printf(`Dryrun enabled. Query operation aborted`)
		return sensu.CheckStateOK, nil
	}
	response, err := doQuery(plugin.Url, plugin.Request, strings.NewReader(plugin.Query))
	log.Printf("http response: %v\n", string(response))
	if err != nil {
		log.Printf("Error attempting query http request: %v", err)
		return sensu.CheckStateCritical, err
	}
	if len(plugin.EvalStatements) > 0 {
		for _, eval := range plugin.EvalStatements {
			result, err := processResponse(string(response), eval)
			log.Printf("eval result: %v\n", result)
			if err != nil {
				log.Printf("Error attempting to evaluate http response: %v", err)
				return sensu.CheckStateCritical, err
			}
			if result != true {
				return sensu.CheckStateWarning, nil
			}
		}
	} else {
		//Do something if there are no eval statement
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
          result = JSON.parse(input)
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
