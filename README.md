# Test Storage Performance of VictoriaMetrics

# Quick Start

## Run VictoriaMetrics

```shell
docker run --name victoria-metrics --rm -d --network host victoriametrics/victoria-metrics -selfScrapeInterval=10s
```

## Run Grafana

```shell
docker run --name grafana --rm -d --network host -v $PWD/grafana/provisioning/:/etc/grafana/provisioning/ -v $PWD/grafana/victoriametrics.json:/var/lib/grafana/dashboards/vm.json grafana/grafana
```

## Run Script

```shell
# Ingest data periodically or
go run main.go

# ingest massive data
go run main.go -begin-ts $((`date +%s`-24*60*60))
```

Click http://127.0.0.1:3000 to see how is going
