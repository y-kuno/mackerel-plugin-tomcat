package mptomcat

import (
	"testing"

	"encoding/json"
	"strings"

	"github.com/stretchr/testify/assert"
)

func TestParseMetrics(t *testing.T) {

	xml := `<?xml version="1.0" encoding="utf-8"?><?xml-stylesheet type="text/xsl" href="/manager/xform.xsl" ?>
<status><jvm><memory free='120588912' total='167247872' max='3817865216'/><memorypool name='PS Eden Space' type='Heap memory' usageInit='67108864' usageCommitted='67108864' usageMax='1409286144' usageUsed='30620784'/><memorypool name='PS Old Gen' type='Heap memory' usageInit='179306496' usageCommitted='89128960' usageMax='2863661056' usageUsed='16038176'/><memorypool name='PS Survivor Space' type='Heap memory' usageInit='11010048' usageCommitted='11010048' usageMax='11010048' usageUsed='0'/><memorypool name='Code Cache' type='Non-heap memory' usageInit='2555904' usageCommitted='8978432' usageMax='251658240' usageUsed='8866688'/><memorypool name='Compressed Class Space' type='Non-heap memory' usageInit='0' usageCommitted='2883584' usageMax='1073741824' usageUsed='2706696'/><memorypool name='Metaspace' type='Non-heap memory' usageInit='0' usageCommitted='26738688' usageMax='-1' usageUsed='25961400'/></jvm><connector name='"ajp-nio-8009"'><threadInfo  maxThreads="200" currentThreadCount="10" currentThreadsBusy="0" /><requestInfo  maxTime="0" processingTime="0" requestCount="0" errorCount="0" bytesReceived="0" bytesSent="0" /><workers></workers></connector><connector name='"http-nio-8080"'><threadInfo  maxThreads="200" currentThreadCount="10" currentThreadsBusy="1" /><requestInfo  maxTime="642" processingTime="1041" requestCount="110" errorCount="6" bytesReceived="0" bytesSent="1096491" /><workers><worker  stage="R" requestProcessingTime="0" requestBytesSent="0" requestBytesReceived="0" remoteAddr="&#63;" virtualHost="&#63;" method="&#63;" currentUri="&#63;" currentQueryString="&#63;" protocol="&#63;" /><worker  stage="R" requestProcessingTime="0" requestBytesSent="0" requestBytesReceived="0" remoteAddr="&#63;" virtualHost="&#63;" method="&#63;" currentUri="&#63;" currentQueryString="&#63;" protocol="&#63;" /><worker  stage="S" requestProcessingTime="2" requestBytesSent="0" requestBytesReceived="0" remoteAddr="0:0:0:0:0:0:0:1" virtualHost="localhost" method="GET" currentUri="/manager/status/all" currentQueryString="XML=true" protocol="HTTP/1.1" /><worker  stage="R" requestProcessingTime="0" requestBytesSent="0" requestBytesReceived="0" remoteAddr="&#63;" virtualHost="&#63;" method="&#63;" currentUri="&#63;" currentQueryString="&#63;" protocol="&#63;" /><worker  stage="R" requestProcessingTime="0" requestBytesSent="0" requestBytesReceived="0" remoteAddr="&#63;" virtualHost="&#63;" method="&#63;" currentUri="&#63;" currentQueryString="&#63;" protocol="&#63;" /></workers></connector></status>`

	var p TomcatPlugin
	metrics := make(map[string]float64)

	err := p.parseMetrics(metrics, []byte(xml))
	if err != nil {
		t.Fatal(err)
	}

	if len(metrics) == 0 {
		t.Fatalf("metrics is empty")
	}

	assert.Equal(t, metrics["free"], float64(120588912))
	assert.Equal(t, metrics["total"], float64(167247872))
	assert.Equal(t, metrics["used"], float64(167247872 - 120588912))

	assert.Equal(t, metrics["thread.ajp.currentThreadsBusy"], float64(0))
	assert.Equal(t, metrics["thread.http.currentThreadsBusy"], float64(1))
}

func TestFetchThreadPool(t *testing.T) {
	str := `{"request":{"mbean":"Catalina:name=*,type=ThreadPool","attribute":"currentThreadsBusy","type":"read"},"value":{"Catalina:name=\"ajp-nio-8009\",type=ThreadPool":{"currentThreadsBusy":123},"Catalina:name=\"http-nio-8080\",type=ThreadPool":{"currentThreadsBusy":345}},"timestamp":1524116731,"status":200}`
/*
{
  "request": {
    "mbean": "Catalina:name=*,type=ThreadPool",
    "attribute": "currentThreadsBusy",
    "type": "read"
  },
  "value": {
    "Catalina:name=\"ajp-nio-8009\",type=ThreadPool": {
      "currentThreadsBusy": 123
    },
    "Catalina:name=\"http-nio-8080\",type=ThreadPool": {
      "currentThreadsBusy": 345
    }
  },
  "timestamp": 1524116737,
  "status": 200
}
 */
	var p TomcatPlugin
	metrics := make(map[string]float64)
	attribute := "currentThreadsBusy"
	var value JolokiaResponse

	buff := strings.NewReader(str)
	dec := json.NewDecoder(buff)
	if err := dec.Decode(&value); err != nil {
		t.Fatal(err)
	}

	err := p.parseThreadPool(attribute, metrics, value)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, metrics["thread.ajp." + attribute], float64(123))
	assert.Equal(t, metrics["thread.http." + attribute], float64(345))
}

func TestFetchGlobalRequestProcessor(t *testing.T) {
	str := `{"request":{"mbean":"Catalina:name=*,type=GlobalRequestProcessor","type":"read"},"value":{"Catalina:name=\"ajp-nio-8009\",type=GlobalRequestProcessor":{"requestCount":0,"maxTime":0,"bytesReceived":0,"modelerType":"org.apache.coyote.RequestGroupInfo","bytesSent":0,"processingTime":0,"errorCount":0},"Catalina:name=\"http-nio-8080\",type=GlobalRequestProcessor":{"requestCount":3,"maxTime":64,"bytesReceived":0,"modelerType":"org.apache.coyote.RequestGroupInfo","bytesSent":6054,"processingTime":83,"errorCount":1}},"timestamp":1524125035,"status":200}`
/*
{
  "request": {
    "mbean": "Catalina:name=*,type=GlobalRequestProcessor",
    "type": "read"
  },
  "value": {
    "Catalina:name=\"ajp-nio-8009\",type=GlobalRequestProcessor": {
      "requestCount": 0,
      "maxTime": 0,
      "bytesReceived": 0,
      "modelerType": "org.apache.coyote.RequestGroupInfo",
      "bytesSent": 0,
      "processingTime": 0,
      "errorCount": 0
    },
    "Catalina:name=\"http-nio-8080\",type=GlobalRequestProcessor": {
      "requestCount": 3,
      "maxTime": 64,
      "bytesReceived": 0,
      "modelerType": "org.apache.coyote.RequestGroupInfo",
      "bytesSent": 6054,
      "processingTime": 83,
      "errorCount": 1
    }
  },
  "timestamp": 1524125095,
  "status": 200
}
 */
	var p TomcatPlugin
	metrics := make(map[string]float64)
	var value JolokiaResponse

	buff := strings.NewReader(str)
	dec := json.NewDecoder(buff)
	if err := dec.Decode(&value); err != nil {
		t.Fatal(err)
	}

	err := p.parseGlobalRequestProcessor( metrics, value)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, metrics["request.processing_time.ajp.processingTime"], float64(0))
	assert.Equal(t, metrics["request.processing_time.http.processingTime"], float64(83))
}
