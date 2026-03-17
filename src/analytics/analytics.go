package analytics

import (
	"log"

	"github.com/posthog/posthog-go"
)

var client posthog.Client
var appVersion string

func Init(apiKey, host, version string) {
	if apiKey == "" {
		return
	}
	cfg := posthog.Config{}
	if host != "" {
		cfg.Endpoint = host
	}
	appVersion = version
	var err error
	client, err = posthog.NewWithConfig(apiKey, cfg)
	if err != nil {
		log.Printf("analytics: failed to init posthog: %v", err)
	}
}

func Close() {
	if client != nil {
		if err := client.Close(); err != nil {
			log.Printf("analytics: close: %v", err)
		}
	}
}

func Track(userID, event string, props map[string]any) {
	if client == nil {
		return
	}
	p := posthog.NewProperties()
	for k, v := range props {
		p.Set(k, v)
	}
	if appVersion != "" {
		p.Set("version", appVersion)
	}
	if err := client.Enqueue(posthog.Capture{
		DistinctId: userID,
		Event:      event,
		Properties: p,
	}); err != nil {
		log.Printf("analytics: track %s: %v", event, err)
	}
}
