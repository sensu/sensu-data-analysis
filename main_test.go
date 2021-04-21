package main

import (
	"fmt"
	"github.com/sensu-community/sensu-plugin-sdk/sensu"
	"strings"
	"testing"
)

func TestMain(t *testing.T) {
}

func TestTLSArguments(t *testing.T) {
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:  "test",
			Short: "test",
		},
	}
	plugin.Url = "https://localhost:80/"
	plugin.EvalStatus = 1
	plugin.TrustedCAFile = "./test/missing-ca.pem"
	t.Run("missing CA File", func(t *testing.T) {
		_, err := checkArgs(nil)
		if err == nil {
			t.Errorf("Missing CA File: no err: %v\n", err)
			return
		}
	})
	plugin.TrustedCAFile = "./test/bad-ca.pem"
	t.Run("bad CA File", func(t *testing.T) {
		_, err := checkArgs(nil)
		if err == nil {
			t.Errorf("Bad CA File: no err: %v\n", err)
			return
		}
	})
	plugin.TrustedCAFile = "./test/ca.pem"
	t.Run("valid CA File", func(t *testing.T) {
		_, err := checkArgs(nil)
		if err != nil {
			t.Errorf("Valid CA File: unexpected error: %v\n", err)
			return
		}
	})

}

func TestServiceUrl(t *testing.T) {
	tests := []struct {
		service_type         string
		expected_default_url string
		expect_error         bool
		override_url         string
		override_host        string
		host_override_url    string
	}{
		{
			service_type:         `prometheus`,
			expected_default_url: `http://localhost:9090/api/v1/query?query=up`,
			expect_error:         false,
			override_url:         `https://example.com:80/path/to/use?param1=val1,param2=val2`,
			override_host:        `other.host.com`,
			host_override_url:    `http://other.host.com:9090/api/v1/query?query=up`,
		},
		{
			service_type:         `unknown`,
			expected_default_url: `http://localhost:9090/api/v1/query?query=up`,
			expect_error:         true,
			override_url:         `https://example.com:80/path/to/use?param1=val1,param2=val2`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.service_type, func(t *testing.T) {
			plugin = Config{
				PluginConfig: sensu.PluginConfig{
					Name:  "test",
					Short: "test",
				},
			}
			//First test that the service default is what is expected
			plugin.Type = tt.service_type
			plugin.Verbose = true
			url, err := finalUrl()
			//log.Printf("finalUrl() expected_default_url: %v url: %v  err: %v\n", tt.expected_default_url, url, err)
			if tt.expect_error {
				if err == nil {
					t.Errorf("finalUrl() Expected return err but got: %v\n", err)
					return
				}
			} else {
				if err != nil {
					t.Errorf("finalUrl() Unexpected return err: %v\n", err)
					return
				} else {
					if strings.Compare(tt.expected_default_url, url) != 0 {
						t.Errorf("finalUrl() Unexpected return url: %v expected_default_url: %v\n", url, tt.expected_default_url)
						return
					}
				}
			}
			if len(tt.override_url) > 0 {
				//test that explicit url override works as expected
				plugin.Url = tt.override_url
				url, err = finalUrl()
				if err != nil {
					t.Errorf("finalUrl() Unexpected return err: %v\n", err)
					return
				} else {
					if strings.Compare(tt.override_url, url) != 0 {
						t.Errorf("finalUrl() Unexpected return url: %v override_url: %v\n", url, tt.override_url)
						return
					}
				}
			}
			plugin.Url = ""
			if len(tt.override_host) > 0 {
				//test that explicit host override works as expected
				plugin.Host = tt.override_host
				url, err = finalUrl()
				if err != nil {
					t.Errorf("finalUrl() Unexpected return err: %v\n", err)
					return
				} else {
					if strings.Compare(tt.host_override_url, url) != 0 {
						t.Errorf("finalUrl() Unexpected return url: %v override_url: %v\n", url, tt.host_override_url)
						return
					}
				}
			}

		})
	}

}
func TestProcess(t *testing.T) {
	tests := []struct {
		name           string
		json_data      string
		jscript        string
		expect_error   bool
		expected_value bool
	}{
		{
			name:           "I could have bought a Lambo",
			json_data:      `{"name":"Frank", "car":"Subaru CrossTrek"}`,
			jscript:        `result.name === "Frank"`,
			expect_error:   false,
			expected_value: true,
		},
		{
			name:           "bad object path",
			json_data:      `{"name":"Frank", "car":"Subaru CrossTrek"}`,
			jscript:        `result.undefined === "Frank"`,
			expect_error:   true,
			expected_value: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processResponse(tt.json_data, tt.jscript)
			if !tt.expect_error && err != nil {
				t.Errorf("processResponse() json_data: %v, jscript: %v, err: %v\n", tt.json_data, tt.jscript, err)
				return
			}
			if result != tt.expected_value {
				t.Errorf("processResponse() json_data: %v, jscript: %v, err: %v\n", tt.json_data, tt.jscript, err)
				return
			}
		})
	}

}
func TestQuery(t *testing.T) {
	plugin.Headers = append(plugin.Headers,
		`First-Header: header value`,
		`Second-Header: second value`,
	)
	tests := []struct {
		name                   string
		url                    string
		request_type           string
		request_data           string
		jscript                string
		expect_query_error     bool
		expect_process_error   bool
		expected_process_value bool
	}{
		{
			name:                   `GET "http://httpbin.org/delay/1"`,
			url:                    `http://httpbin.org/delay/1`,
			request_type:           `GET`,
			request_data:           "",
			expect_query_error:     false,
			expect_process_error:   false,
			expected_process_value: false,
			jscript:                ``,
		},
		{
			name:                   `POST "https://httpbin.org/post"`,
			url:                    `http://httpbin.org/post`,
			request_type:           `POST`,
			request_data:           `{"test":"value"}`,
			expect_query_error:     false,
			expect_process_error:   false,
			expected_process_value: true,
			jscript:                `result.json.test === "value"`,
		},
		{
			name:                   `POST "https://httpbin.org/post"`,
			url:                    `http://httpbin.org/post`,
			request_type:           `POST`,
			request_data:           `{"test":"value"}`,
			expect_query_error:     false,
			expect_process_error:   false,
			expected_process_value: false,
			jscript:                `result.json.test === "bad value"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := doQuery(tt.url, tt.request_type, strings.NewReader(tt.request_data))
			if !tt.expect_query_error && err != nil {
				t.Errorf("doQuery() url: %v, body: %v err: %v\n", tt.url, string(result), err)
				return
			}
			presult, err := processResponse(string(result), `result.headers["First-Header"] === "header value" && result.headers["Second-Header"] === "second value"`)
			if err != nil {
				t.Errorf("processResponse() json_data: %v, jscript: %v, err: %v\n", string(result), tt.jscript, err)
				t.Errorf("doQuery() url: %v, body: %v err: %v\n", tt.url, string(result), err)
				return
			}
			if presult != true {
				t.Errorf("Unexpeted header value json_data: %v, err: %v\n", string(result), err)
				t.Errorf("doQuery() url: %v, body: %v err: %v\n", tt.url, string(result), err)
				return
			}
			presult, err = processResponse(string(result), tt.jscript)
			if !tt.expect_process_error && err != nil {
				t.Errorf("processResponse() json_data: %v, jscript: %v, err: %v\n", string(result), tt.jscript, err)
				t.Errorf("doQuery() url: %v, body: %v err: %v\n", tt.url, string(result), err)
				return
			}
			if presult != tt.expected_process_value {
				t.Errorf("processResponse() json_data: %v, jscript: %v, err: %v\n", string(result), tt.jscript, err)
				t.Errorf("doQuery() url: %v, body: %v err: %v\n", tt.url, string(result), err)
				return
			}
		})
	}

}

func TestMultipleEval(t *testing.T) {
	evalStatus := 10
	plugin = Config{
		PluginConfig: sensu.PluginConfig{
			Name:  "test",
			Short: "test",
		},
	}
	plugin.Url = `http://httpbin.org/post`
	plugin.Request = `POST`
	plugin.Debug = false
	plugin.Verbose = false
	plugin.EvalStatus = evalStatus
	statements := [](string){
		"result.url",
	}
	t.Run("test no eval", func(t *testing.T) {
		status, err := executeCheck(nil)
		if status != 0 {
			t.Errorf("executeCheck(nil) status: %v err: %v", status, err)
			return
		}
	})
	plugin.EvalStatements = statements
	fmt.Printf("testing eval statements: %v\n", statements)
	t.Run("test 1 true eval", func(t *testing.T) {
		status, err := executeCheck(nil)
		if status != 0 {
			t.Errorf("executeCheck(nil) status: %v err: %v", status, err)
			return
		}
	})
	statements = [](string){
		"result.json",
	}
	plugin.EvalStatements = statements
	fmt.Printf("testing eval statements: %v\n", plugin.EvalStatements)
	t.Run("test 1 false eval with default status", func(t *testing.T) {
		status, err := executeCheck(nil)
		if status != evalStatus {
			t.Errorf("executeCheck(nil) status: %v err: %v", status, err)
			return
		}
	})
	statements = [](string){
		"result.url",
		"result.json",
	}
	plugin.EvalStatements = statements
	fmt.Printf("testing eval statements: %v\n", statements)
	t.Run("test 1 true 1 false eval with eval status 10", func(t *testing.T) {
		status, err := executeCheck(nil)
		if status != evalStatus {
			t.Errorf("executeCheck(nil) status: %v err: %v", status, err)
			return
		}
	})
	statements = [](string){
		"result.url",
		"result.origin",
	}
	plugin.EvalStatements = statements
	fmt.Printf("testing eval statements: %v\n", statements)
	t.Run("test 2 true eval", func(t *testing.T) {
		status, err := executeCheck(nil)
		if status != 0 {
			t.Errorf("executeCheck(nil) status: %v err: %v", status, err)
			return
		}
	})
}
