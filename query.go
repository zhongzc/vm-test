package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

var (
	workerCount = flag.Uint("worker-count", 1, "Count of workers")
	freshness   = flag.String("freshness", "1h", "How fresh are the data")
	timeRange   = flag.String("time-range", "5m", "Time range of queries")
	sumWindow   = flag.String("sum-window", "1m", "Window to sum over datapoints")
	instanceSet = flag.String("instance-set", "0-2,4-5", "Instance set")
	url         = flag.String("vm-url", "http://localhost:8428/api/v1/query_range", "vm query url")

	freshnessSecs int64 = 0
	timeRangeSecs int64 = 0
	sumWindowSecs int64 = 0
	instanceList  []int
)

func main() {
	flag.Parse()
	parseDuration()
	parseInstanceList()

	rand.Seed(time.Now().Unix())
	for i := uint(0); i < *workerCount; i++ {
		go runQueries()
	}

	logStat()
}

func runQueries() {
	for {
		now := time.Now().Unix()
		end := now - rand.Int63n(freshnessSecs)
		end = end - end%sumWindowSecs
		start := end - timeRangeSecs
		step := sumWindowSecs

		instance := instanceList[rand.Intn(len(instanceList))]
		query := fmt.Sprintf("sum(label_replace(topk(5, sum_over_time(cpu_time{instance=\"tikv-%d\"}[%s])), \"digest\", \"$1\", \"tag\", \"(.*)\") * on(digest) group_left(sql) sql_digest{}) by (instance, sql)", instance, *sumWindow)

		if req, err := http.NewRequest("GET", *url, nil); err != nil {
			log.Panic(err)
		} else {
			q := req.URL.Query()
			q.Add("query", query)
			q.Add("start", strconv.Itoa(int(start)))
			q.Add("end", strconv.Itoa(int(end)))
			q.Add("step", strconv.Itoa(int(step)))
			req.URL.RawQuery = q.Encode()

			now := time.Now()
			if resp, err := http.DefaultClient.Do(req); err != nil {
				log.Panic(err)
			} else {
				if _, err := ioutil.ReadAll(resp.Body); err != nil {
					log.Panic(err)
				}

				if err := resp.Body.Close(); err != nil {
					log.Panic(err)
				}

				lock.Lock()
				_ = hist.RecordValue(int64(time.Now().Nanosecond() - now.Nanosecond()))
				lock.Unlock()
				atomic.AddInt64(&metricsWrittenCount, 1)
			}
		}
	}
}

var (
	metricsWrittenCount int64 = 0

	lock sync.Mutex
	hist = hdrhistogram.New(1, 30_000_000_000_000, 4)
)

func logStat() {
	prev := atomic.LoadInt64(&metricsWrittenCount)
	prevTs := time.Now().Unix()

	for {
		time.Sleep(5 * time.Second)
		cur := atomic.LoadInt64(&metricsWrittenCount)
		nowTs := time.Now().Unix()

		lock.Lock()
		cp := hist.Export()
		hist.Reset()
		lock.Unlock()

		hist := hdrhistogram.Import(cp)
		log.Printf("QPS: %.2f, p50: %.2fms, p99: %.2fms, avg: %.2fms\n",
			float64(cur-prev)/float64(nowTs-prevTs),
			float64(hist.ValueAtQuantile(50.0))/1000000,
			float64(hist.ValueAtQuantile(99.0))/1000000,
			float64(hist.Mean())/1000000,
		)
		prevTs = nowTs
		prev = cur
	}
}

func parseDuration() {
	if d, err := time.ParseDuration(*freshness); err != nil {
		log.Panic(err)
	} else {
		freshnessSecs = int64(d.Seconds())
	}
	if d, err := time.ParseDuration(*timeRange); err != nil {
		log.Panic(err)
	} else {
		timeRangeSecs = int64(d.Seconds())
	}
	if d, err := time.ParseDuration(*sumWindow); err != nil {
		log.Panic(err)
	} else {
		sumWindowSecs = int64(d.Seconds())
	}
}

func parseInstanceList() {
	is := *instanceSet
	if len(is) == 0 {
		log.Panic("Instance set can not be empty")
	}

	for _, s := range strings.Split(is, ",") {
		lr := strings.Split(s, "-")
		if len(lr) == 1 {
			if n, err := strconv.Atoi(lr[0]); err != nil {
				log.Panic(err)
			} else {
				instanceList = append(instanceList, n)
			}
		} else if len(lr) == 2 {
			var l, r int
			if n, err := strconv.Atoi(lr[0]); err != nil {
				log.Panic(err)
			} else {
				l = n
			}
			if n, err := strconv.Atoi(lr[1]); err != nil {
				log.Panic(err)
			} else {
				r = n
			}
			if l >= r {
				log.Panicf("%d should be less than %d", lr[0], lr[1])
			}
			for i := l; i <= r; i++ {
				instanceList = append(instanceList, i)
			}
		} else {
			log.Panicf("unexpected instance set: %s", is)
		}
	}
}
