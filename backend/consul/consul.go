package consul

import (
	api "github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
)

// Backend ...
type Backend struct {
	client *api.Client
}

// NewBackend ...
func NewBackend() *Backend {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		log.Fatalf("Can not create Consul client: %s", err)
	}

	_, err = client.Status().Leader()
	if err != nil {
		log.Fatalf("Could not connect to Consul client: %s", err)
	}

	return &Backend{client}
}
