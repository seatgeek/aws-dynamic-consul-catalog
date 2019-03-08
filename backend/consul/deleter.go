package consul

import (
	api "github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
)

// DeleteService ...
func (b *Backend) DeleteService(service, node string) {
	_, err := b.client.Catalog().Deregister(&api.CatalogDeregistration{
		Node:      node,
		ServiceID: service,
	}, &api.WriteOptions{})

	if err != nil {
		log.Errorf("Could not delete consul service %s for node %s: %s", service, node, err)
	}
}

// DeleteCheck ...
func (b *Backend) DeleteCheck(check, node string) {
	_, err := b.client.Catalog().Deregister(&api.CatalogDeregistration{
		Node:    node,
		CheckID: check,
	}, &api.WriteOptions{})

	if err != nil {
		log.Errorf("Could not delete consul check %s for node %s: %s", check, node, err)
	}
}
