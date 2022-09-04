package graf

import "embed"

// TODO: need to dynamically add grafana_datasources.yml to point to prometheus IP that might change

//go:embed Dockerfile grafana.ini grafana_datasources.yml
var GrafanaSource embed.FS
