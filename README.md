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

### Load Usage
```
-begin-ts int
    begin unix timestamp in second. Can be set an early ts to import tons of data, or a recent ts to import data per minute. (default 1629360783)
-instance-count uint
    instance count (default 1000)
-tag-count uint
    tag count per second (default 200)
-vm-url string
    vm import url (default "http://localhost:8428/api/v1/import")
-gzip
    compress import requests by gzip
```

### Run Load
```shell
# Ingest data periodically or
go run load.go

# ingest massive data
go run load.go -begin-ts $((`date +%s`-24*60*60))
```

### Query Usage
```
-freshness string
    How fresh are the data (default "1h")
-instance-count uint
    instance count (default 1000)
-sum-window string
    Window to sum over datapoints (default "1m")
-time-range string
    Time range of queries (default "5m")
-vm-url string
    vm query url (default "http://localhost:8428/api/v1/query_range")
-workers uint
    Count of workers (default 1)
```

Click http://127.0.0.1:3000 to see how is going
