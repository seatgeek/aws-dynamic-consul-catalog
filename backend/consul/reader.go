package consul

import (
	"time"

	consul "github.com/hashicorp/consul/api"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

// internalNode ...
type internalNode struct {
	Node            string
	Address         string
	TaggedAddresses map[string]string
	Services        []*consul.AgentService
	Checks          []*consul.AgentCheck
}

// CatalogReader ...
func (b *Backend) CatalogReader(state *config.CatalogState, consulNodeName string, quitCh chan int) {
	logger := log.WithField("consul", "CatalogReader")
	logger.Debug("Starting Consul catalog reader")

	raw := b.client.Raw()

	q := &consul.QueryOptions{
		WaitIndex: 1,
		WaitTime:  120 * time.Second,
	}

	for {
		select {
		case <-quitCh:
			return

		default:
			logger.Infof("Waiting for %s information to change", consulNodeName)

			var newNode internalNode

			meta, err := raw.Query("/v1/internal/ui/node/"+consulNodeName, &newNode, q)
			if err != nil {
				time.Sleep(30 * time.Second)
				continue
			}

			remoteWaitIndex := meta.LastIndex
			localWaitIndex := q.WaitIndex

			if remoteWaitIndex == localWaitIndex {
				logger.Infof("Wait index is unchanged (%d == %d)", localWaitIndex, remoteWaitIndex)
				continue
			}

			logger.Infof("Wait index is changed (%d <> %d)", localWaitIndex, remoteWaitIndex)
			q.WaitIndex = remoteWaitIndex

			state.Lock()
			state.Services = processCatalog(newNode)
			state.Unlock()
		}
	}
}

func processCatalog(n internalNode) config.Services {
	services := make(config.Services)

	for _, service := range n.Services {
		services[service.ID] = &config.Service{
			ServiceID:      service.ID,
			ServiceName:    service.Service,
			ServiceTags:    service.Tags,
			ServiceAddress: service.Address,
			ServicePort:    service.Port,
			ServiceMeta:    service.Meta,
		}
	}

	for _, check := range n.Checks {
		if check.CheckID == "serfHealth" {
			continue
		}

		if _, ok := services[check.ServiceID]; !ok {
			log.Errorf("Could not find a service '%s' for check '%s'", check.ServiceID, check.CheckID)
		}

		services[check.ServiceID].CheckID = check.CheckID
		services[check.ServiceID].CheckNode = check.Node
		services[check.ServiceID].CheckStatus = check.Status
		services[check.ServiceID].CheckOutput = check.Output
		services[check.ServiceID].CheckNotes = check.Notes
	}

	return services
}
