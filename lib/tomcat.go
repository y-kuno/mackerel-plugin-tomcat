package mptomcat

import (
	"flag"
	"os"

	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	mp "github.com/mackerelio/go-mackerel-plugin"
)

// TomcatPlugin mackerel plugin
type TomcatPlugin struct {
	Host     string
	Port     string
	User     string
	Password string
	Module   string
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

// JolokiaResponse tomcat jolokia response struct
type JolokiaResponse struct {
	Request   map[string]interface{}
	Value     map[string]interface{}
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
			Label: labelPrefix + " Threads",
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
			Label: labelPrefix + " Request Counts",
			Unit:  "integer",
			Metrics: []mp.Metrics{
				{Name: "requestCount", Label: "request", Diff: true},
				{Name: "errorCount", Label: "error", Diff: true},
			},
		},
		"request.byte.#": {
			Label: labelPrefix + " Request Bytes",
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
	if p.Module == "jolokia" {
		return p.fetchJolokiaMetrics()
	}

	return p.fetchManagerMetrics()
}

func (p *TomcatPlugin) fetchManagerMetrics() (map[string]float64, error) {
	metrics := make(map[string]float64)

	client := http.DefaultClient
	url := fmt.Sprintf("http://%s:%s/manager/status/all?XML=true", p.Host, p.Port)
	req, err := http.NewRequest("GET", url, nil)
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

func (p *TomcatPlugin) fetchJolokiaMetrics() (map[string]float64, error) {
	metrics := make(map[string]float64)

	if err := p.fetchThreadPool(metrics); err != nil {
		return nil, err
	}
	if err := p.fetchGlobalRequestProcessor(metrics); err != nil {
		return nil, err
	}

	return metrics, nil
}

func (p *TomcatPlugin) fetchThreadPool(metrics map[string]float64) error {
	var attributes = []string{"maxThreads", "currentThreadCount", "currentThreadsBusy"}
	for _, attr := range attributes {
		res, err := p.executeGetRequest(fmt.Sprintf("Catalina:name=*,type=ThreadPool/%s", attr))
		if err != nil {
			return err
		}

		if err := p.parseThreadPool(attr, metrics, res); err != nil {
			return err
		}
	}

	return nil
}

func (p *TomcatPlugin) parseThreadPool(attribute string, metrics map[string]float64, res JolokiaResponse) error {
	for k, v := range res.Value {
		value := v.(map[string]interface{})
		protocol := strings.Split(strings.Split(k, "\"")[1], "-")[0]
		// thread
		metrics["thread." + protocol + "." + attribute] = value[attribute].(float64)
	}

	return nil
}

func (p *TomcatPlugin) fetchGlobalRequestProcessor(metrics map[string]float64) error {
	res, err := p.executeGetRequest("Catalina:name=*,type=GlobalRequestProcessor")
	if err != nil {
		return err
	}

	if err := p.parseGlobalRequestProcessor(metrics, res); err != nil {
		return err
	}

	return nil
}

func (p *TomcatPlugin) parseGlobalRequestProcessor(metrics map[string]float64, res JolokiaResponse) error {
	for k, v := range res.Value {
		value := v.(map[string]interface{})
		protocol := strings.Split(strings.Split(k, "\"")[1], "-")[0]
		// processing time
		metrics["request.processing_time." + protocol + ".maxTime"] = value["maxTime"].(float64)
		metrics["request.processing_time." + protocol + ".processingTime"] = value["processingTime"].(float64)
		// request count
		metrics["request.count." + protocol + ".requestCount"] = value["requestCount"].(float64)
		metrics["request.count." + protocol + ".errorCount"] = value["errorCount"].(float64)
		// request byte
		metrics["request.byte." + protocol + ".bytesReceived"] = value["bytesReceived"].(float64)
		metrics["request.byte." + protocol + ".bytesSent"] = value["bytesSent"].(float64)
	}

	return nil
}

func (p *TomcatPlugin) executeGetRequest(mbean string) (JolokiaResponse, error) {
	var jolokiaResponse JolokiaResponse

	url := fmt.Sprintf("http://%s:%s/jolokia/read/%s", p.Host, p.Port, mbean)
	res, err := http.Get(url)
	if err != nil {
		return jolokiaResponse ,err
	}
	defer res.Body.Close()

	dec := json.NewDecoder(res.Body)
	if err := dec.Decode(&jolokiaResponse); err != nil {
		return jolokiaResponse, err
	}

	return jolokiaResponse, nil
}

// Do the plugin
func Do() {
	optHost := flag.String("host", "localhost", "Hostname")
	optPort := flag.String("port", "8080", "Port")
	optUser := flag.String("user", "tomcat", "Username")
	optPassword := flag.String("password", os.Getenv("TOMCAT_PASSWORD"), "Password")
	optModule := flag.String("module", "", "Module")
	optPrefix := flag.String("metric-key-prefix", "tomcat", "Metric key prefix")
	optTempfile := flag.String("tempfile", "", "Temp file name")
	flag.Parse()

	plugin := mp.NewMackerelPlugin(&TomcatPlugin{
		Host: *optHost,
		Port: *optPort,
		User: *optUser,
		Password: *optPassword,
		Module: *optModule,
		Prefix: *optPrefix,
	})
	plugin.Tempfile = *optTempfile
	plugin.Run()
}