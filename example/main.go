package main

import (
	"context"
	"log"
	"time"

	"github.com/zenoss/zenoss-protobufs/go/cloud/data_receiver"

	"github.com/zenoss/zenoss-go-sdk/endpoint"
	"github.com/zenoss/zenoss-go-sdk/metadata"
)

func main() {
	epConfig := endpoint.Config{
		Name:   "zenoss",
		APIKey: "XXXXXXX",
	}

	ep, err := endpoint.New(epConfig)
	if err != nil {
		log.Fatal(err)
	}

	statusResult, err := ep.PutMetrics(context.Background(), &data_receiver.Metrics{
		DetailedResponse: true,
		Metrics: []*data_receiver.Metric{
			{
				Metric:    "example.com/things_count",
				Timestamp: time.Now().UnixNano() / 1e6, // Timestamp must be Unix milliseconds.
				Value:     123.4,
				Dimensions: map[string]string{
					"source": "com.example.things.app",
				},
				MetadataFields: metadata.FromStringMap(map[string]string{
					"source-type": "zenoss.io/zenoss-go-sdk/example",
				}),
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}

	if statusResult.GetFailed() > 0 {
		log.Print("failed to send metric")
	} else {
		log.Print("sent metric")
	}

	modelStatusResult, err := ep.PutModels(context.Background(), &data_receiver.Models{
		Models: []*data_receiver.Model{
			{
				Timestamp: time.Now().UnixNano() / 1e6,
				Dimensions: map[string]string{
					"source": "com.example.things.app",
				},
				MetadataFields: metadata.FromStringMap(map[string]string{
					"source-type": "zenoss.io/zenoss-go-sdk/example",
					"name":        "Things App",
				}),
			},
		},
	})

	if err != nil {
		log.Fatal(err)
	}

	if modelStatusResult.GetFailed() > 0 {
		log.Print("failed to send model")
	} else {
		log.Print("sent model")
	}
}
