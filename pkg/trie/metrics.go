package trie

import (
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"time"
)

type metric struct {
	name      string
	help      string
	valueType prometheus.ValueType
	getValue  func(t *Trie) float64
}

func (m *metric) desc(baseLabels []string) *prometheus.Desc {
	return prometheus.NewDesc(m.name, m.help, baseLabels, nil)
}

var (
	allMetrics = []metric{
		{
			name:      "http_requests_total",
			help:      "Total number of HTTP requests",
			valueType: prometheus.CounterValue,
			getValue: func(t *Trie) float64 {
				return float64(t.count)
			},
		},
		{
			name:      "http_requests_avg_duration_milliseconds",
			help:      "Duration of HTTP requests",
			valueType: prometheus.GaugeValue,
			getValue: func(t *Trie) float64 {
				return t.avgDuration / 1e6
			},
		},
	}
)

func (t *Trie) Labels() map[string]string {
	routes := strings.Split(t.route, " ")
	var method, path string
	if len(routes) >= 2 {
		method = routes[0]
		path = routes[1]
	}
	return map[string]string{
		"method": method,
		"path":   path,
	}
}

func (t *Trie) Collect(ch chan<- prometheus.Metric) {
	allTries := t.GetAll()
	for _, trie := range allTries {
		rawLabels := map[string]struct{}{}
		for l := range trie.Labels() {
			rawLabels[l] = struct{}{}
		}
		values := make([]string, 0, len(rawLabels))
		labels := make([]string, 0, len(rawLabels))
		metricLabels := trie.Labels()
		for l := range rawLabels {
			duplicate := false
			for _, x := range labels {
				if l == x {
					duplicate = true
					break
				}
			}
			if !duplicate {
				labels = append(labels, l)
				values = append(values, metricLabels[l])
			}
		}
		for _, m := range allMetrics {
			ch <- prometheus.NewMetricWithTimestamp(
				time.Now(),
				prometheus.MustNewConstMetric(m.desc(labels), m.valueType, m.getValue(trie), values...),
			)
		}
	}
}

func (t *Trie) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range allMetrics {
		ch <- m.desc([]string{})
	}
}
