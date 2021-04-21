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
	"net/http"
	"net/url"
	"strings"
	"time"

	corev2 "github.com/sensu/sensu-go/api/core/v2"
)

// Config represents the check plugin config.
type Config struct {
	sensu.PluginConfig
	//low level arguments for any http request:
	Timeout            int
	Headers            []string
	Request            string
	Url                string
	EvalStatements     []string
	EvalStatus         int
	Query              string
	Type               string
	Verbose            bool
	Debug              bool
	DryRun             bool
	Scheme             string
	Host               string
	Port               int
	ApiPath            string
	ApiParams          string
	TrustedCAFile      string
	InsecureSkipVerify bool
	MTLSKeyFile        string
	MTLSCertFile       string
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
		"influxdb": ServiceType{
			Scheme:    "http",
			Host:      "localhost",
			Port:      8086,
			ApiPath:   "query",
			ApiParams: "db=sensu",
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
			Short:    "The Sensu Data Analysis plugin queries data platforms via HTTP APIs and evaluates JSON responses using Javascript conditional expressions (--eval). JS expressions are evaluated in a \"sandbox\" that is seeded with a single variable called 'result' representing the complete query response in JSON format.",
			Keyspace: "sensu.io/plugins/sensu-metric-analysis/config",
		},
	}

	options = []*sensu.PluginConfigOption{
		&sensu.PluginConfigOption{
			Path:      "",
			Env:       "",
			Argument:  "timeout",
			Shorthand: "T",
			Default:   15,
			Usage:     "Request timeout in seconds",
			Value:     &plugin.Timeout,
		},
		&sensu.PluginConfigOption{
			Path:     "",
			Env:      "",
			Argument: "result-status",
			Default:  1,
			Usage:    "Check result status if any eval statement condition is not met (eg. a metric exceeds a threshold). Must be >= 1.",
			Value:    &plugin.EvalStatus,
		},
		&sensu.PluginConfigOption{
			Path:      "",
			Env:       "",
			Argument:  "url",
			Shorthand: "U",
			Default:   "",
			Usage:     "API URL to use (e.g.: https://httpbin.org/post). All other URL component arguments are ignored if provided.",
			Value:     &plugin.Url,
		},
		&sensu.PluginConfigOption{
			Path:      "",
			Env:       "",
			Argument:  "request",
			Shorthand: "r",
			Default:   "",
			Usage:     "Default to \"get\" unless --query is set, it defaults to \"post\"",
			Value:     &plugin.Request,
		},
		{
			Path:      "",
			Env:       "",
			Argument:  "header",
			Shorthand: "H",
			Default:   []string{},
			Usage:     "HTTP request header(s). Note: some headers may be preset if --type is provided.",
			Value:     &plugin.Headers,
		},
		&sensu.PluginConfigOption{
			Path:      "",
			Env:       "",
			Argument:  "eval",
			Shorthand: "e",
			Default:   []string{},
			Usage:     "Array of Javascript expressions that must return a bool. If no eval is provided, the check will return the query response as standard output. Ex: result.test === \"value\"",
			Value:     &plugin.EvalStatements,
		},
		&sensu.PluginConfigOption{
			Path:      "",
			Env:       "",
			Argument:  "query",
			Shorthand: "q",
			Default:   "",
			Usage:     "Query expression.",
			Value:     &plugin.Query,
		},
		&sensu.PluginConfigOption{
			Path:      "",
			Env:       "",
			Argument:  "type",
			Shorthand: "t",
			Default:   "",
			Usage:     "Optional (no default is set). Sets --request, --header, --port, --path, and --params based on the backend type (e.g. prometheus, elasticsearch, or influxdb). Setting --type=prometheus",
			Value:     &plugin.Type,
		},
		&sensu.PluginConfigOption{
			Path:      "",
			Env:       "",
			Argument:  "verbose",
			Shorthand: "v",
			Default:   false,
			Usage:     "Enable verbose output",
			Value:     &plugin.Verbose,
		},
		&sensu.PluginConfigOption{
			Path:      "",
			Env:       "",
			Argument:  "debug",
			Shorthand: "",
			Default:   false,
			Usage:     "Enable debug output",
			Value:     &plugin.Verbose,
		},
		&sensu.PluginConfigOption{
			Path:      "",
			Env:       "",
			Argument:  "dryrun",
			Shorthand: "n",
			Default:   false,
			Usage:     "Do not execute query, just report configuration. Useful for diagnostic testing.",
			Value:     &plugin.DryRun,
		},
		&sensu.PluginConfigOption{
			Path:      "",
			Env:       "",
			Argument:  "scheme",
			Shorthand: "",
			Usage:     "HTTP request scheme (http or https).",
			Value:     &plugin.Scheme,
		},
		&sensu.PluginConfigOption{
			Path:     "host",
			Argument: "host",
			Usage:    "HTTP request hostname (or IP address).",
			Value:    &plugin.Host,
		},
		&sensu.PluginConfigOption{
			Path:     "port",
			Argument: "port",
			Usage:    "HTTP request port number.",
			Value:    &plugin.Port,
		},
		&sensu.PluginConfigOption{
			Path:     "path",
			Argument: "path",
			Usage:    "HTTP request path (e.g. \"api/v1/query\"",
			Value:    &plugin.ApiPath,
		},
		&sensu.PluginConfigOption{
			Path:     "params",
			Argument: "params",
			Usage:    "HTTP request params (e.g. \"db=sensu\")",
			Value:    &plugin.ApiParams,
		},
		{
			Argument: "insecure-skip-verify",
			Default:  false,
			Usage:    "Skip TLS certificate verification (not recommended!)",
			Value:    &plugin.InsecureSkipVerify,
		},
		{
			Argument: "trusted-ca-file",
			Default:  "",
			Usage:    "TLS CA certificate bundle in PEM format",
			Value:    &plugin.TrustedCAFile,
		},
		{
			Argument: "mtls-key-file",
			Default:  "",
			Usage:    "Key file for mutual TLS auth in PEM format",
			Value:    &plugin.MTLSKeyFile,
		},
		{
			Argument: "mtls-cert-file",
			Default:  "",
			Usage:    "Certificate file for mutual TLS auth in PEM format",
			Value:    &plugin.MTLSCertFile,
		},
	}
)

func main() {
	check := sensu.NewGoCheck(&plugin.PluginConfig, options, checkArgs, executeCheck, false)
	check.Execute()
}

func serviceDefaults(service ServiceType) {
	if plugin.Debug {
		fmt.Printf("Setting service defaults for provider: %s\n", plugin.Type)
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
			if plugin.Debug {
				fmt.Printf("Found supported service type: %v\n", plugin.Type)
			}
			serviceDefaults(service)
		} else {
			if plugin.Verbose {
				fmt.Printf("Unknown Service Type: %v\n", plugin.Type)
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
	if plugin.Debug {
		plugin.Verbose = true
	}
	if plugin.DryRun {
		plugin.Verbose = true
		plugin.Debug = true
	}
	newUrl, err := finalUrl()
	plugin.Url = newUrl

	if len(plugin.Request) == 0 {
		plugin.Request = `GET`
	}

	if plugin.Debug {
		fmt.Printf("  Type: %v\n", plugin.Type)
		fmt.Printf("  Request Method: %v\n", plugin.Request)
		fmt.Printf("  Url: %v\n", plugin.Url)
		fmt.Printf("  Trusted CA File: %v\n", plugin.TrustedCAFile)
		fmt.Printf("  Skip TLS Verify: %v\n", plugin.InsecureSkipVerify)
		fmt.Printf("  MTLS Cert File: %v\n", plugin.MTLSCertFile)
		fmt.Printf("  MTLS Key File: %v\n", plugin.MTLSKeyFile)
		fmt.Printf("  Headers: %v\n", plugin.Headers)
		fmt.Printf("  Query: %v\n", plugin.Query)
		fmt.Printf("  Eval Statements: %v\n", plugin.EvalStatements)
		fmt.Printf("\n")
		fmt.Printf("Available service types:\n")
		for name, service := range supportedServices {
			fmt.Printf("  %v: %v\n", name, service)
		}
		fmt.Printf("\n")
	}

	if err != nil {
		if plugin.DryRun {
			fmt.Printf("Warning: unexpected error associated with Url: %v", err)
		} else {
			return sensu.CheckStateWarning, err
		}
	}

	if plugin.EvalStatus < 1 {
		if plugin.DryRun {
			fmt.Printf("Warning: -eval-status >= 1 is required")
		} else {
			return sensu.CheckStateWarning, fmt.Errorf("--eval-status >= 1 is required")
		}
	}

	if len(plugin.TrustedCAFile) > 0 {
		caCertPool, err := corev2.LoadCACerts(plugin.TrustedCAFile)
		if err != nil {
			return sensu.CheckStateWarning, fmt.Errorf("Error loading specified CA file")
		}
		tlsConfig.RootCAs = caCertPool
	}
	tlsConfig.InsecureSkipVerify = plugin.InsecureSkipVerify
	tlsConfig.CipherSuites = corev2.DefaultCipherSuites

	if (len(plugin.MTLSKeyFile) > 0 && len(plugin.MTLSCertFile) == 0) || (len(plugin.MTLSCertFile) > 0 && len(plugin.MTLSKeyFile) == 0) {
		return sensu.CheckStateWarning, fmt.Errorf("mTLS auth requires both --mtls-key-file and --mtls-cert-file")
	}
	if len(plugin.MTLSKeyFile) > 0 && len(plugin.MTLSCertFile) > 0 {
		cert, err := tls.LoadX509KeyPair(plugin.MTLSCertFile, plugin.MTLSKeyFile)
		if err != nil {
			return sensu.CheckStateWarning, fmt.Errorf("Failed to load mTLS key pair %s/%s: %v", plugin.MTLSCertFile, plugin.MTLSKeyFile, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return sensu.CheckStateOK, nil
}

func executeCheck(event *types.Event) (int, error) {
	if plugin.DryRun {
		fmt.Printf(`Dryrun enabled. Query operation aborted`)
		return sensu.CheckStateOK, nil
	}
	response, err := doQuery(plugin.Url, plugin.Request, strings.NewReader(plugin.Query))
	if plugin.Debug {
		fmt.Printf("http response: %v\n", string(response))
	}
	if err != nil {
		fmt.Printf("Error attempting query http request: %v\n", err)
		return sensu.CheckStateCritical, err
	}
	if len(plugin.EvalStatements) > 0 {
		// Loop over eval statements
		// return on first error or first false eval statement
		for _, eval := range plugin.EvalStatements {
			result, err := processResponse(string(response), eval)
			if plugin.Debug {
				fmt.Printf("Eval result: %v (%s)\n", result, eval)
			}
			//return if eval statement throws error
			if err != nil {
				fmt.Printf("Error attempting to evaluate http response: %v\n", err)
				return sensu.CheckStateCritical, err
			}
			//return if eval statement result is false
			if !result {
				fmt.Printf("An eval condition was not met: \"%s\" (%v)\n", eval, result)
				if plugin.Verbose {
					fmt.Printf("\n%s\n", string(response))
				}
				return plugin.EvalStatus, nil
			}
		}
		// If all eval statements result to true
		fmt.Printf("All eval condition were met.\n")
		if plugin.Verbose {
			fmt.Printf("\n%s\n", string(response))
		}
	} else {
		if plugin.Verbose {
			fmt.Printf("No eval statements present. Returning query result: %v\n", string(response))
		} else {
			fmt.Printf("%v\n", string(response))
		}
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

	err := vm.Set("input", data)
	if err != nil {
		fmt.Printf("vm.Set error: %v", err)
		return false, err
	}
	_, err = vm.Run(`
          result = JSON.parse(input)
        `)
	if err != nil {
		fmt.Printf("vm.Run error: %v", err)
		return false, err
	}
	return_value, err := vm.Run(jscript)
	if err != nil {
		fmt.Printf("vm.Run error: %v", err)
		return false, err
	} else {
		if return_bool, err := return_value.ToBoolean(); err == nil {
			if err != nil {
				fmt.Printf("return_value.ToBoolean error: %v\n", err)
				return return_bool, err
			} else {
				return return_bool, err
			}
		}
	}
	return false, fmt.Errorf("processResponse: unknown error")
}
