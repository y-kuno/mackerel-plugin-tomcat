package mptomcat

import (
	"flag"
	"os"

	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	mp "github.com/mackerelio/go-mackerel-plugin"
)

// TomcatPlugin mackerel plugin
type TomcatPlugin struct {
	Target   string
	User     string
	Password string
	Prefix   string
}

// Status tomcat xml status struct
type Status struct {
	Jvm       Jvm         `xml:"jvm"`
	Connector []Connector `xml:"connector"`
}

// Jvm tomcat xml status struct
type Jvm struct {
	Memory struct {
		Free  float64 `xml:"free,attr"`
		Total float64 `xml:"total,attr"`
		Max   float64 `xml:"max,attr"`
	} `xml:"memory"`
}

// Connector tomcat xml connector struct
type Connector struct {
	Name        string      `xml:"name,attr"`
	ThreadInfo  ThreadInfo  `xml:"threadInfo"`
	RequestInfo RequestInfo `xml:"requestInfo"`
}

// ThreadInfo tomcat xml threadInfo struct
type ThreadInfo struct {
	MaxThreads         float64 `xml:"maxThreads,attr"`
	CurrentThreadCount float64 `xml:"currentThreadCount,attr"`
	CurrentThreadsBusy float64 `xml:"currentThreadsBusy,attr"`
}

// RequestInfo tomcat xml requestInfo struct
type RequestInfo struct {
	MaxTime        float64 `xml:"maxTime,attr"`
	ProcessingTime float64 `xml:"processingTime,attr"`
	RequestCount   float64 `xml:"requestCount,attr"`
	ErrorCount     float64 `xml:"errorCount,attr"`
	BytesReceived  float64 `xml:"bytesReceived,attr"`
	BytesSent      float64 `xml:"bytesSent,attr"`
}

// MetricKeyPrefix interface for PluginWithPrefix
func (p *TomcatPlugin) MetricKeyPrefix() string {
	if p.Prefix == "" {
		p.Prefix = "tomcat"
	}
	return p.Prefix
}

// GraphDefinition interface for mackerelplugin
func (p *TomcatPlugin) GraphDefinition() map[string]mp.Graphs {
	labelPrefix := strings.Title(p.Prefix)
	return map[string]mp.Graphs{
		"jvm.memory": {
			Label: labelPrefix + " Jvm Memory",
			Unit:  "bytes",
			Metrics: []mp.Metrics{
				{Name: "free", Label: "free", Stacked: true},
				{Name: "used", Label: "used", Stacked: true},
				{Name: "total", Label: "total"},
				{Name: "max", Label: "max"},
			},
		},
		"thread.#": {
			Label: labelPrefix + " Thread",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "maxThreads", Label: "max"},
				{Name: "currentThreadCount", Label: "current"},
				{Name: "currentThreadsBusy", Label: "busy"},
			},
		},
		"request.processing_time.#": {
			Label: labelPrefix + " Request Processing Time",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "maxTime", Label: "max"},
				{Name: "processingTime", Label: "processing", Diff: true},
			},
		},
		"request.count.#": {
			Label: labelPrefix + " Request Count",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "requestCount", Label: "request", Diff: true},
				{Name: "errorCount", Label: "error", Diff: true},
			},
		},
		"request.byte.#": {
			Label: labelPrefix + " Request Byte",
			Unit:  "bytes",
			Metrics: []mp.Metrics{
				{Name: "bytesReceived", Label: "received", Diff: true},
				{Name: "bytesSent", Label: "sent", Diff: true},
			},
		},
	}
}

// FetchMetrics interface for mackerelplugin
func (p *TomcatPlugin) FetchMetrics() (map[string]float64, error) {
	metrics := make(map[string]float64)

	client := http.DefaultClient
	req, err := http.NewRequest("GET", p.Target, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(p.User, p.Password)
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if err := p.parseMetrics(metrics, body); err != nil {
		return nil, err
	}

	return metrics, nil
}

func (p *TomcatPlugin) parseMetrics(metrics map[string]float64, data []byte) error {
	var status Status
	if err := xml.Unmarshal(data, &status); err != nil {
		return err
	}

	metrics["free"] = status.Jvm.Memory.Free
	metrics["total"] = status.Jvm.Memory.Total
	metrics["used"] = metrics["total"] - metrics["free"]
	metrics["max"] = status.Jvm.Memory.Max

	for _, connector := range status.Connector {
		array := strings.Split(connector.Name, "-")
		protocol := strings.Trim(array[0], "\"")
		// thread
		metrics["thread." + protocol + ".maxThreads"] = connector.ThreadInfo.MaxThreads
		metrics["thread." + protocol + ".currentThreadCount"] = connector.ThreadInfo.CurrentThreadCount
		metrics["thread." + protocol + ".currentThreadsBusy"] = connector.ThreadInfo.CurrentThreadsBusy
		// processing time
		metrics["request.processing_time." + protocol + ".maxTime"] = connector.RequestInfo.MaxTime
		metrics["request.processing_time." + protocol + ".processingTime"] = connector.RequestInfo.ProcessingTime
		// request count
		metrics["request.count." + protocol + ".requestCount"] = connector.RequestInfo.RequestCount
		metrics["request.count." + protocol + ".errorCount"] = connector.RequestInfo.ErrorCount
		// request byte
		metrics["request.byte." + protocol + ".bytesReceived"] = connector.RequestInfo.BytesReceived
		metrics["request.byte." + protocol + ".bytesSent"] = connector.RequestInfo.BytesSent
	}

	return nil
}


// Do the plugin
func Do() {
	optHost := flag.String("host", "localhost", "Hostname")
	optPort := flag.String("port", "8080", "Port")
	optUser := flag.String("user", "tomcat", "Username")
	optPassword := flag.String("password", os.Getenv("TOMCAT_PASSWORD"), "Password")
	optPrefix := flag.String("metric-key-prefix", "tomcat", "Metric key prefix")
	optTempfile := flag.String("tempfile", "", "Temp file name")
	flag.Parse()

	plugin := mp.NewMackerelPlugin(&TomcatPlugin{
		Target: fmt.Sprintf("http://%s:%s/manager/status/all?XML=true", *optHost, *optPort),
		User: *optUser,
		Password: *optPassword,
		Prefix: *optPrefix,
	})
	plugin.Tempfile = *optTempfile
	plugin.Run()
}