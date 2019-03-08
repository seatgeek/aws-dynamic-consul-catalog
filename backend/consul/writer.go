package consul

import (
	api "github.com/hashicorp/consul/api"
	"github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

// WriteService ...
func (b *Backend) WriteService(service *config.Service) {
	save := &api.CatalogRegistration{
		Node:    service.CheckNode,
		Address: service.ServiceAddress,
		Service: &api.AgentService{
			Address: service.ServiceAddress,
			ID:      service.ServiceID,
			Port:    service.ServicePort,
			Service: service.ServiceName,
			Tags:    service.ServiceTags,
			Meta:    service.ServiceMeta,
		},
		Check: &api.AgentCheck{
			CheckID:     service.CheckID,
			Name:        service.ServiceName,
			Node:        service.CheckNode,
			Notes:       service.CheckNotes,
			ServiceName: service.ServiceName,
			ServiceID:   service.ServiceID,
			Status:      service.CheckStatus,
			Output:      service.CheckOutput,
		},
	}

	_, err := b.client.Catalog().Register(save, &api.WriteOptions{})

	if err != nil {
		log.Errorf("Could not write consul catalog: %s", err)
	}
}
