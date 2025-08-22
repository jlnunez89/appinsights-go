package appinsights

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const DefaultIngestionEndpoint = "https://in.applicationinsights.azure.com"

// Configuration data used to initialize a new TelemetryClient.
type TelemetryConfiguration struct {
	// Instrumentation key for the client.
	InstrumentationKey string

	// Endpoint URL where data will be submitted.
	EndpointUrl string

	// Application ID associated with the Application Insights resource.
	ApplicationId string

	// Maximum number of telemetry items that can be submitted in each
	// request.  If this many items are buffered, the buffer will be
	// flushed before MaxBatchInterval expires.
	MaxBatchSize int

	// Maximum time to wait before sending a batch of telemetry.
	MaxBatchInterval time.Duration

	// Customized http client if desired (will use http.DefaultClient otherwise)
	Client *http.Client

	// Sampling processor for controlling telemetry volume (optional)
	SamplingProcessor SamplingProcessor

	// Error auto-collection configuration (optional)
	ErrorAutoCollection *ErrorAutoCollectionConfig

	// Automatic event collection configuration (optional)
	AutoCollection *AutoCollectionConfig
}

// Creates a new TelemetryConfiguration object with the specified
// connection string and default values.
func NewTelemetryConfiguration(connectionString string) *TelemetryConfiguration {
	ikey, ingestionEndpoint, appId, err := parseConnectionString(connectionString)

	if err != nil {
		panic(err)
	}

	return &TelemetryConfiguration{
		InstrumentationKey: ikey,
		EndpointUrl:        ingestionEndpoint,
		ApplicationId:      appId,
		MaxBatchSize:       1024,
		MaxBatchInterval:   time.Duration(10) * time.Second,
	}
}

func parseConnectionString(connectionString string) (string, string, string, error) {
	parts := map[string]string{}
	for _, part := range splitAndTrim(connectionString, ";") {
		kv := splitAndTrim(part, "=")
		if len(kv) == 2 {
			parts[kv[0]] = kv[1]
		}
	}

	ikey, ok := parts["InstrumentationKey"]
	if !ok || ikey == "" {
		return "", "", "", fmt.Errorf("missing or empty InstrumentationKey")
	}

	endpoint, ok := parts["IngestionEndpoint"]
	if !ok || endpoint == "" {
		endpoint = DefaultIngestionEndpoint
	}

	appId := parts["ApplicationId"]

	return ikey, endpoint, appId, nil
}

func splitAndTrim(connectionString, s string) []string {
	parts := []string{}
	for _, part := range strings.Split(connectionString, s) {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts

}

func (config *TelemetryConfiguration) setupContext() *TelemetryContext {
	context := NewTelemetryContext(config.InstrumentationKey)
	context.Tags.Internal().SetSdkVersion(sdkName + ":" + Version)
	context.Tags.Device().SetOsVersion(runtime.GOOS)

	if hostname, err := os.Hostname(); err == nil {
		context.Tags.Device().SetId(hostname)
		context.Tags.Cloud().SetRoleInstance(hostname)
	}

	return context
}
