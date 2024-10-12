package rds

import (
	"reflect"
	"regexp"

	observer "github.com/imkira/go-observer"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

var removeUpdatedTimeRegexp = regexp.MustCompile("\n\nLast update: .+")

func (r *RDS) writer(prop observer.Property, state *config.CatalogState) {
	logger := log.WithField("rds", "writer")
	logger.Info("Starting RDS Consul Catalog writer")

	stream := prop.Observe()

	for {
		select {
		case <-r.quitCh:
			return

		// wait for changes
		case <-stream.Changes():
			state.Lock()

			logger.Debug("Starting Consul Catalog write")

			for stream.HasNext() {
				stream.Next()
				switch v := stream.Value().(type) {
				case []*config.RDSInstances:
					instances := stream.Value().([]*config.RDSInstances)
					seen := state.Services.GetSeen()

					found := &config.SeenCatalog{
						Services: make([]string, 0),
						Checks:   make([]string, 0),
					}

					for _, instance := range instances {
						r.writeBackendCatalogInstances(instance, logger, state, found)
					}
					for _, service := range r.getDifference(seen.Services, found.Services) {
						logger.Warnf("Deleting service %s", service)
						r.backend.DeleteService(service, r.consulNodeName)
					}

					for _, check := range r.getDifference(seen.Checks, found.Checks) {
						logger.Warnf("Deleting check %s", check)
						r.backend.DeleteCheck(check, r.consulNodeName)
					}
				// case []*config.RDSClusters:
				// 	clusters := stream.Value().([]*config.RDSClusters)
				// 	seen := state.Services.GetSeen()

				// 	found := &config.SeenCatalog{
				// 		Services: make([]string, 0),
				// 		Checks:   make([]string, 0),
				// 	}

				// 	for _, cluster := range clusters {
				// 		r.writeBackendCatalogClusters(cluster, logger, state, found)
				// 	}
				// 	for _, service := range r.getDifference(seen.Services, found.Services) {
				// 		logger.Warnf("Deleting service %s", service)
				// 		r.backend.DeleteService(service, r.consulNodeName)
				// 	}

				// 	for _, check := range r.getDifference(seen.Checks, found.Checks) {
				// 		logger.Warnf("Deleting check %s", check)
				// 		r.backend.DeleteCheck(check, r.consulNodeName)
				// 	}
				// case []*config.RDSGlobalCluster:
				// 	globalclusters := stream.Value().([]*config.RDSGlobalCluster)
				// 	seen := state.Services.GetSeen()

				// 	found := &config.SeenCatalog{
				// 		Services: make([]string, 0),
				// 		Checks:   make([]string, 0),
				// 	}

				// 	for _, globalcluster := range globalclusters {
				// 		r.writeBackendCatalogGlobalClusters(globalcluster, logger, state, found)
				// 	}
				// 	for _, service := range r.getDifference(seen.Services, found.Services) {
				// 		logger.Warnf("Deleting service %s", service)
				// 		r.backend.DeleteService(service, r.consulNodeName)
				// 	}

				// 	for _, check := range r.getDifference(seen.Checks, found.Checks) {
				// 		logger.Warnf("Deleting check %s", check)
				// 		r.backend.DeleteCheck(check, r.consulNodeName)
				// 	}
				default:
					log.Printf("Unexpected type %T", v)
				}
			}

			logger.Debug("Finished Consul Catalog write")

			state.Unlock()
		}
	}
}

func (r *RDS) identicalService(a, b *config.Service, logger *log.Entry) bool {
	if a.ServiceID != b.ServiceID {
		logger.Infof("ServiceID are not identical (%s vs %s)", a.ServiceID, b.ServiceID)
		return false
	}

	if a.ServiceName != b.ServiceName {
		logger.Infof("ServiceName are not identical (%s vs %s)", a.ServiceName, b.ServiceName)
		return false
	}

	if a.ServiceAddress != b.ServiceAddress {
		logger.Infof("ServiceAddress are not identical (%s vs %s)", a.ServiceAddress, b.ServiceAddress)
		return false
	}

	if a.ServicePort != b.ServicePort {
		logger.Infof("ServicePort are not identical (%d vs %d)", a.ServicePort, b.ServicePort)
		return false
	}

	if a.CheckNotes != b.CheckNotes {
		logger.Infof("CheckNotes are not identical (%s vs %s)", a.CheckNotes, b.CheckNotes)
		return false
	}

	if a.CheckStatus != b.CheckStatus {
		logger.Infof("CheckStatus are not identical (%s vs %s)", a.CheckStatus, b.CheckStatus)
		return false
	}

	if !reflect.DeepEqual(a.ServiceMeta, b.ServiceMeta) {
		logger.Infof("ServiceMeta are not identical (%+v vs %+v)", a.ServiceMeta, b.ServiceMeta)
		return false
	}

	if removeUpdatedTimeRegexp.ReplaceAllLiteralString(a.CheckOutput, "") != removeUpdatedTimeRegexp.ReplaceAllLiteralString(b.CheckOutput, "") {
		logger.Infof("CheckOutput are not identical (%+v vs %+v)", a.CheckOutput, b.CheckOutput)
		return false
	}

	if r.isDifferent(a.ServiceTags, b.ServiceTags) {
		logger.Infof("ServiceTags are not identical (%+v vs %+v)", a.ServiceTags, b.ServiceTags)
		return false
	}

	return true
}

func (r *RDS) getDifference(slice1, slice2 []string) []string {
	diff := make([]string, 0)

	for _, s1 := range slice1 {
		found := false

		for _, s2 := range slice2 {
			if s1 == s2 {
				found = true
				break
			}
		}

		if !found {
			diff = append(diff, s1)
		}
	}

	return diff
}

func (r *RDS) isDifferent(slice1, slice2 []string) bool {
	if len(r.getDifference(slice1, slice2)) > 0 {
		return true
	}

	return len(r.getDifference(slice2, slice1)) > 0
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
