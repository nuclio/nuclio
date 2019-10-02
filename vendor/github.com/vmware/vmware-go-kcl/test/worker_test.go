/*
 * Copyright (c) 2018 VMware, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
 * associated documentation files (the "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is furnished to do
 * so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all copies or substantial
 * portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT
 * NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
 * WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */
package test

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/prometheus/common/expfmt"
	log "github.com/sirupsen/logrus"

	"github.com/stretchr/testify/assert"
	cfg "github.com/vmware/vmware-go-kcl/clientlibrary/config"
	kc "github.com/vmware/vmware-go-kcl/clientlibrary/interfaces"
	"github.com/vmware/vmware-go-kcl/clientlibrary/metrics"
	"github.com/vmware/vmware-go-kcl/clientlibrary/utils"
	wk "github.com/vmware/vmware-go-kcl/clientlibrary/worker"
)

const (
	streamName = "kcl-test"
	regionName = "us-west-2"
	workerID   = "test-worker"
)

const specstr = `{"name":"kube-qQyhk","networking":{"containerNetworkCidr":"10.2.0.0/16"},"orgName":"BVT-Org-cLQch","projectName":"project-tDSJd","serviceLevel":"DEVELOPER","size":{"count":1},"version":"1.8.1-4"}`
const metricsSystem = "cloudwatch"

var shardID string

func TestWorker(t *testing.T) {
	kclConfig := cfg.NewKinesisClientLibConfig("appName", streamName, regionName, workerID).
		WithInitialPositionInStream(cfg.LATEST).
		WithMaxRecords(10).
		WithMaxLeasesForWorker(1).
		WithShardSyncIntervalMillis(5000).
		WithFailoverTimeMillis(300000).
		WithMetricsBufferTimeMillis(10000).
		WithMetricsMaxQueueSize(20)

	runTest(kclConfig, false, t)
}

func TestWorkerWithSigInt(t *testing.T) {
	kclConfig := cfg.NewKinesisClientLibConfig("appName", streamName, regionName, workerID).
		WithInitialPositionInStream(cfg.LATEST).
		WithMaxRecords(10).
		WithMaxLeasesForWorker(1).
		WithShardSyncIntervalMillis(5000).
		WithFailoverTimeMillis(300000).
		WithMetricsBufferTimeMillis(10000).
		WithMetricsMaxQueueSize(20)

	runTest(kclConfig, true, t)
}

func TestWorkerStatic(t *testing.T) {
	t.Skip("Need to provide actual credentials")

	creds := credentials.NewStaticCredentials("AccessKeyId", "SecretAccessKey", "")

	kclConfig := cfg.NewKinesisClientLibConfigWithCredential("appName", streamName, regionName, workerID, creds).
		WithInitialPositionInStream(cfg.LATEST).
		WithMaxRecords(10).
		WithMaxLeasesForWorker(1).
		WithShardSyncIntervalMillis(5000).
		WithFailoverTimeMillis(300000).
		WithMetricsBufferTimeMillis(10000).
		WithMetricsMaxQueueSize(20)

	runTest(kclConfig, false, t)
}

func TestWorkerAssumeRole(t *testing.T) {
	t.Skip("Need to provide actual roleARN")

	// Initial credentials loaded from SDK's default credential chain. Such as
	// the environment, shared credentials (~/.aws/credentials), or EC2 Instance
	// Role. These credentials will be used to to make the STS Assume Role API.
	sess := session.Must(session.NewSession())

	// Create the credentials from AssumeRoleProvider to assume the role
	// referenced by the "myRoleARN" ARN.
	creds := stscreds.NewCredentials(sess, "arn:aws:iam::*:role/kcl-test-publisher")

	kclConfig := cfg.NewKinesisClientLibConfigWithCredential("appName", streamName, regionName, workerID, creds).
		WithInitialPositionInStream(cfg.LATEST).
		WithMaxRecords(10).
		WithMaxLeasesForWorker(1).
		WithShardSyncIntervalMillis(5000).
		WithFailoverTimeMillis(300000).
		WithMetricsBufferTimeMillis(10000).
		WithMetricsMaxQueueSize(20)

	runTest(kclConfig, false, t)
}

func runTest(kclConfig *cfg.KinesisClientLibConfiguration, triggersig bool, t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	assert.Equal(t, regionName, kclConfig.RegionName)
	assert.Equal(t, streamName, kclConfig.StreamName)

	// configure cloudwatch as metrics system
	metricsConfig := getMetricsConfig(kclConfig, metricsSystem)

	worker := wk.NewWorker(recordProcessorFactory(t), kclConfig, metricsConfig)

	err := worker.Start()
	assert.Nil(t, err)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Signal processing.
	go func() {
		sig := <-sigs
		t.Logf("Received signal %s. Exiting", sig)
		worker.Shutdown()
		// some other processing before exit.
		//os.Exit(0)
	}()

	// Put some data into stream.
	t.Log("Putting data into stream.")
	for i := 0; i < 100; i++ {
		// Use random string as partition key to ensure even distribution across shards
		err := worker.Publish(streamName, utils.RandStringBytesMaskImpr(10), []byte(specstr))
		if err != nil {
			t.Errorf("Errorin Publish. %+v", err)
		}
	}
	t.Log("Done putting data into stream.")

	if triggersig {
		t.Log("Trigger signal SIGINT")
		p, _ := os.FindProcess(os.Getpid())
		p.Signal(os.Interrupt)
	}

	// wait a few seconds before shutdown processing
	time.Sleep(10 * time.Second)

	if metricsConfig != nil && metricsConfig.MonitoringService == "prometheus" {
		res, err := http.Get("http://localhost:8080/metrics")
		if err != nil {
			t.Fatalf("Error scraping Prometheus endpoint %s", err)
		}

		var parser expfmt.TextParser
		parsed, err := parser.TextToMetricFamilies(res.Body)
		res.Body.Close()
		if err != nil {
			t.Errorf("Error reading monitoring response %s", err)
		}
		t.Logf("Prometheus: %+v", parsed)

	}

	t.Log("Calling normal shutdown at the end of application.")
	worker.Shutdown()
}

// configure different metrics system
func getMetricsConfig(kclConfig *cfg.KinesisClientLibConfiguration, service string) *metrics.MonitoringConfiguration {
	if service == "cloudwatch" {
		return &metrics.MonitoringConfiguration{
			MonitoringService: "cloudwatch",
			Region:            regionName,
			CloudWatch: metrics.CloudWatchMonitoringService{
				Credentials: kclConfig.CloudWatchCredentials,
				// Those value should come from kclConfig
				MetricsBufferTimeMillis: kclConfig.MetricsBufferTimeMillis,
				MetricsMaxQueueSize:     kclConfig.MetricsMaxQueueSize,
			},
		}
	}

	if service == "prometheus" {
		return &metrics.MonitoringConfiguration{
			MonitoringService: "prometheus",
			Region:            regionName,
			Prometheus: metrics.PrometheusMonitoringService{
				ListenAddress: ":8080",
			},
		}
	}

	return nil
}

// Record processor factory is used to create RecordProcessor
func recordProcessorFactory(t *testing.T) kc.IRecordProcessorFactory {
	return &dumpRecordProcessorFactory{t: t}
}

// simple record processor and dump everything
type dumpRecordProcessorFactory struct {
	t *testing.T
}

func (d *dumpRecordProcessorFactory) CreateProcessor() kc.IRecordProcessor {
	return &dumpRecordProcessor{
		t: d.t,
	}
}

// Create a dump record processor for printing out all data from record.
type dumpRecordProcessor struct {
	t *testing.T
}

func (dd *dumpRecordProcessor) Initialize(input *kc.InitializationInput) {
	dd.t.Logf("Processing SharId: %v at checkpoint: %v", input.ShardId, aws.StringValue(input.ExtendedSequenceNumber.SequenceNumber))
	shardID = input.ShardId
}

func (dd *dumpRecordProcessor) ProcessRecords(input *kc.ProcessRecordsInput) {
	dd.t.Log("Processing Records...")

	// don't process empty record
	if len(input.Records) == 0 {
		return
	}

	for _, v := range input.Records {
		dd.t.Logf("Record = %s", v.Data)
		assert.Equal(dd.t, specstr, string(v.Data))
	}

	// checkpoint it after processing this batch
	lastRecordSequenceNubmer := input.Records[len(input.Records)-1].SequenceNumber
	dd.t.Logf("Checkpoint progress at: %v,  MillisBehindLatest = %v", lastRecordSequenceNubmer, input.MillisBehindLatest)
	input.Checkpointer.Checkpoint(lastRecordSequenceNubmer)
}

func (dd *dumpRecordProcessor) Shutdown(input *kc.ShutdownInput) {
	dd.t.Logf("Shutdown Reason: %v", aws.StringValue(kc.ShutdownReasonMessage(input.ShutdownReason)))

	// When the value of {@link ShutdownInput#getShutdownReason()} is
	// {@link com.amazonaws.services.kinesis.clientlibrary.lib.worker.ShutdownReason#TERMINATE} it is required that you
	// checkpoint. Failure to do so will result in an IllegalArgumentException, and the KCL no longer making progress.
	if input.ShutdownReason == kc.TERMINATE {
		input.Checkpointer.Checkpoint(nil)
	}
}
