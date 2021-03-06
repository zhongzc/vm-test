package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

var (
	beginTimestamp    = flag.Int64("begin-ts", time.Now().Unix(), "begin unix timestamp in second. Can be set an early ts to import tons of data, or a recent ts to import data per minute.")
	tagCount          = flag.Uint("tag-count", 200, "tag count per second")
	instanceCount     = flag.Uint("instance-count", 1000, "instance count")
	updateTagInterval = flag.String("update-tag-interval", "1m", "interval to update tags")
	url               = flag.String("vm-url", "http://localhost:8428/api/v1/import", "vm import url")
	gz                = flag.Bool("gzip", false, "compress import requests by gzip")
)

type Metrics struct {
	Metric     map[string]string `json:"metric"`
	Values     []int64           `json:"values"`
	Timestamps []int64           `json:"timestamps"`
}

type Tag struct {
	Tag string
	SQL string
}

var tags []Tag

func main() {
	flag.Parse()
	tags = make([]Tag, *tagCount)
	parseDuration()
	go logThroughput()
	importDataToVM()
}

func importDataToVM() {
	reportTs := *beginTimestamp
	for {
		nowTs := time.Now().Unix()
		if reportTs > nowTs {
			time.Sleep(time.Duration(reportTs-nowTs) * time.Second)
		}
		updateTags(reportTs)

		var buf bytes.Buffer

		var gzipw *gzip.Writer
		var jsonw *json.Encoder
		if *gz {
			gzipw = gzip.NewWriter(&buf)
			jsonw = json.NewEncoder(gzipw)
		} else {
			jsonw = json.NewEncoder(&buf)
		}

		for _, tag := range tags {
			m := Metrics{Metric: map[string]string{
				"__name__": "sql_digest",
				"digest":   tag.Tag,
				"sql":      tag.SQL,
			}}
			m.Timestamps = append(m.Timestamps, reportTs*1000)
			m.Values = append(m.Values, 1)
			if err := jsonw.Encode(m); err != nil {
				log.Panic(err)
			}
			atomic.AddInt64(&metricsWrittenCount, 1)

			for ins := uint(0); ins < *instanceCount; ins++ {
				m := Metrics{Metric: map[string]string{
					"__name__": "cpu_time",
					"tag":      tag.Tag,
					"instance": fmt.Sprintf("tikv-%d", ins),
				}}
				for ts := reportTs - 60; ts < reportTs; ts++ {
					m.Timestamps = append(m.Timestamps, ts*1000)
					m.Values = append(m.Values, rand.Int63n(100))
					atomic.AddInt64(&metricsWrittenCount, 1)
				}
				if err := jsonw.Encode(m); err != nil {
					log.Panic(err)
				}
			}
		}

		if *gz {
			if err := gzipw.Close(); err != nil {
				log.Panic(err)
			}
		}

		req, err := http.NewRequest("POST", *url, &buf)
		if err != nil {
			log.Panic(err)
		}
		if *gz {
			req.Header.Set("Content-Encoding", "gzip")
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Panic(err)
		}

		if err := resp.Body.Close(); err != nil {
			log.Panic(err)
		}

		reportTs += 60
	}
}

var (
	updateTagIntervalSecs int64 = 0
	lastUpdateTimeSecs    int64 = 0
)

func updateTags(reportSecs int64) {
	if reportSecs-lastUpdateTimeSecs >= updateTagIntervalSecs {
		for i := 0; i < len(tags); i++ {
			tags[i] = Tag{
				Tag: uuid.New().String(),
				SQL: fmt.Sprintf("SELECT COUNT(?) FROM t_%d_%d", reportSecs, i),
			}
		}
		lastUpdateTimeSecs = reportSecs
	}
}

var metricsWrittenCount int64 = 0

func logThroughput() {
	prev := atomic.LoadInt64(&metricsWrittenCount)
	prevTs := time.Now().Unix()

	for {
		time.Sleep(5 * time.Second)
		cur := atomic.LoadInt64(&metricsWrittenCount)
		nowTs := time.Now().Unix()
		log.Printf("Load rate: %d metrics/s\n", (cur-prev)/(nowTs-prevTs))
		prevTs = nowTs
		prev = cur
	}
}

func parseDuration() {
	if d, err := time.ParseDuration(*updateTagInterval); err != nil {
		log.Panic(err)
	} else {
		updateTagIntervalSecs = int64(d.Seconds())
	}
}
