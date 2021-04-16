package main

import (
	"testing"
)

func TestMain(t *testing.T) {
}

func TestQuery(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		request_type   string
		expect_error   bool
		expected_value string
	}{
		{
			name:           `GET "http://httpbin.org/delay/1"`,
			url:            `http://httpbin.org/delay/1`,
			request_type:   `GET`,
			expect_error:   false,
			expected_value: "hey",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := doQuery(tt.url, tt.request_type, "")
			if !tt.expect_error && err != nil {
				t.Errorf("doQuery() url: %v, body: %v err: %v\n", tt.url, string(result), err)
				return
			}
			if string(result) != tt.expected_value {
				t.Errorf("doQuery() url: %v, body: %v err: %v\n", tt.url, string(result), err)
				return
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
			jscript:        `data.name === "Frank"`,
			expect_error:   false,
			expected_value: true,
		},
		{
			name:           "bad object path",
			json_data:      `{"name":"Frank", "car":"Subaru CrossTrek"}`,
			jscript:        `data.undefined === "Frank"`,
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
