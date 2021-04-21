[![Sensu Bonsai Asset](https://img.shields.io/badge/Bonsai-Download%20Me-brightgreen.svg?colorB=89C967&logo=sensu)](https://bonsai.sensu.io/assets/sensu/sensu-data-analysis)
![Go Test](https://github.com/sensu/sensu-data-analysis/workflows/Go%20Test/badge.svg)
![goreleaser](https://github.com/sensu/sensu-data-analysis/workflows/goreleaser/badge.svg)

# Sensu Data Analysis

## Table of Contents

- [Overview](#overview)
- [Usage](#usage)
- [Configuration](#configuration)
  - [Asset registration](#asset-registration)
  - [Check definition](#check-definition)
- [Installation from source](#installation-from-source)
- [Additional notes](#additional-notes)
- [Contributing](#contributing)

## Overview

The Sensu Data Analysis plugin queries data platforms via HTTP APIs and evaluates JSON responses using Javascript conditional expressions (--eval), triggering Sensu event handlers (including alerts) based on trends in time-series and other observability data.
The Sensu Data Analysis plugin enables Sensu users to configure metric and other data anslysis jobs (checks) alongside Sensu Go's already extensive data collection capabilities – as code.

## Usage

```
The Sensu Data Analysis plugin queries data platforms via HTTP APIs and evaluates JSON responses using Javascript conditional expressions (--eval). JS expressions are evaluated in a "sandbox" that is seeded with a single variable called 'result' representing the complete query response in JSON format.

Usage:
  sensu-data-analysis [flags]
  sensu-data-analysis [command]

Available Commands:
  help        Help about any command
  version     Print the version number of this plugin

Flags:
      --debug                    Enable debug output
  -n, --dryrun                   Do not execute query, just report configuration. Useful for diagnostic testing.
  -e, --eval strings             Array of Javascript expressions that must return a bool. If no eval is provided, the check will return the query response as standard output. Ex: result.test === "value"
  -H, --header strings           HTTP request header(s). Note: some headers may be preset if --type is provided.
  -h, --help                     help for sensu-data-analysis
      --host string              HTTP request hostname (or IP address).
      --insecure-skip-verify     Skip TLS certificate verification (not recommended!)
      --mtls-cert-file string    Certificate file for mutual TLS auth in PEM format
      --mtls-key-file string     Key file for mutual TLS auth in PEM format
      --params string            HTTP request params (e.g. "db=sensu")
      --path string              HTTP request path (e.g. "api/v1/query"
      --port int                 HTTP request port number.
  -q, --query string             Query expression.
  -r, --request string           Default to "get" unless --query is set, it defaults to "post"
      --result-status int        Check result status if any eval statement condition is not met (eg. a metric exceeds a threshold). Must be >= 1. (default 1)
      --scheme string            HTTP request scheme (http or https).
  -T, --timeout int              Request timeout in seconds (default 15)
      --trusted-ca-file string   TLS CA certificate bundle in PEM format
  -t, --type string              Optional (no default is set). Sets --request, --header, --port, --path, and --params based on the backend type (e.g. prometheus, elasticsearch, or influxdb). Setting --type=prometheus
  -U, --url string               API URL to use (e.g.: https://httpbin.org/post). All other URL component arguments are ignored if provided.
  -v, --verbose                  Enable verbose output

Use "sensu-data-analysis [command] --help" for more information about a command.
```

## Supported data providers

The Sensu Data Analysis plugin works with any restful HTTP API that returns JSON.
For convenience, templates are provided for the following data providers (as set via the `--type` flag).

**`prometheus`**

Setting `--type=prometheus` provides the following defaults:

- `--scheme="http"`
- `--host="localhost"`
- `--port="9090"`
- `--path="api/v1/query"`
- `--params="query=up"`
- `--request="POST"`
- `--header="Content-Type: application/x-www-form-urlencoded"`

Please see the [Prometheus HTTP API "Expression queries" documentation](https://prometheus.io/docs/prometheus/latest/querying/api/#expression-queries) for more information.

**`influxdb` (InfluxQL)**

Setting `--type=influxdb` provides the following defaults:

- `--scheme="http"`
- `--host="localhost"`
- `--port="8086"`
- `--path="query"`
- `--params="db=sensu"`
- `--request="POST"`
- `--header="Content-Type: application/x-www-form-urlencoded"`

Please see the [InfluxDB API "Query data with InfluxQL" documentation](https://docs.influxdata.com/influxdb/v1.8/guides/query_data/#query-data-with-influxql) for more information.

> **NOTE:** support for additional built-in data providers is coming soon, including:
>
> - Elasticsearch (Search API)
> - Elasticsearch (Query API)
> - Splunk
> - Sumo Logic
> - Cloudwatch
> - Graphite
> - Wavefront
>
> In the interim, the `sensu-data-analysis` plugin should "just work" ™️ with most or all of these providers, given the correct parameters (e.g. `--url`, `--header`s, etc).
> Please let us know of any data platforms you'd like to see built-in support for by [opening an issue](https://github.com/sensu/sensu-data-analysis/issues/new) (or commenting with a +1 an an [existing issue](https://github.com/sensu/sensu-data-analysis/issues)).

## Configuration

### Asset registration

[Sensu Assets][10] are the best way to make use of this plugin. If you're not using an asset, please
consider doing so! If you're using sensuctl 5.13 with Sensu Backend 5.13 or later, you can use the
following command to add the asset:

```
sensuctl asset add sensu/sensu-data-analysis
```

If you're using an earlier version of sensuctl, you can find the asset on the [Bonsai Asset Index][https://bonsai.sensu.io/assets/sensu/sensu-data-analysis].

### Check definition

```yml
---
type: CheckConfig
api_version: core/v2
metadata:
  name: example-metric-analysis
spec:
  command: >-
    sensu-data-analysis
    --type influxdb
    --query 'q=SELECT MEAN("response_time") FROM "app_metrics" WHERE time > now() - 1h'
    --eval '!!result.results'
    --eval 'result.results[0].series[0].values[0][1] < 0.001'
    --result-status 1
  runtime_assets:
  - sensu/sensu-data-analysis:0.2.0
  publish: true
  subscriptions:
  - influxdb
  interval: 300
  timeout: 10
```

## Installation from source

The preferred way of installing and deploying this plugin is to use it as an Asset.
If you would like to compile and install the plugin from source or contribute to it, download the latest version or create an executable script from this source.

From the local path of the sensu-data-analysis repository:

```
go build
```

## Additional notes

## Contributing

For more information about contributing to this plugin, see [Contributing][1].
And don't forget to [register your contribution](https://sensu.io/register-your-contribution) – no matter how small – to get FREE SWAG!

[1]: https://github.com/sensu/sensu-go/blob/master/CONTRIBUTING.md
[2]: https://github.com/sensu/sensu-plugin-sdk
[3]: https://github.com/sensu-plugins/community/blob/master/PLUGIN_STYLEGUIDE.md
[4]: https://github.com/sensu/check-plugin-template/blob/master/.github/workflows/release.yml
[5]: https://github.com/sensu/check-plugin-template/actions
[6]: https://docs.sensu.io/sensu-go/latest/reference/checks/
[7]: https://github.com/sensu/check-plugin-template/blob/master/main.go
[8]: https://bonsai.sensu.io/
[9]: https://github.com/sensu/sensu-plugin-tool
[10]: https://docs.sensu.io/sensu-go/latest/reference/assets/
