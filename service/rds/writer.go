package rds

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"time"

	aws "github.com/aws/aws-sdk-go-v2/aws"
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

			stream.Next()
			instances := stream.Value().([]*config.RDSInstances)
			// clusters := stream.Value().([]*config.RDSClusters)
			// globalclusters := stream.Value().([]*config.RDSGlobalCluster)

			seen := state.Services.GetSeen()

			found := &config.SeenCatalog{
				Services: make([]string, 0),
				Checks:   make([]string, 0),
			}

			for _, instance := range instances {
				r.writeBackendCatalog(instance, logger, state, found)
			}

			for _, service := range r.getDifference(seen.Services, found.Services) {
				logger.Warnf("Deleting service %s", service)
				r.backend.DeleteService(service, r.consulNodeName)
			}

			for _, check := range r.getDifference(seen.Checks, found.Checks) {
				logger.Warnf("Deleting check %s", check)
				r.backend.DeleteCheck(check, r.consulNodeName)
			}

			logger.Debug("Finished Consul Catalog write")

			state.Unlock()
		}
	}
}

func (r *RDS) writeBackendCatalog(instance *config.RDSInstances, logger *log.Entry, state *config.CatalogState, seen *config.SeenCatalog) {
	logger.Info("Starting RDS Consul writeBackendCatalog")

	name := r.getServiceName(instance)
	if name == "" {
		return
	}
	id := name

	if *instance.RDSInstance.DBInstanceStatus == "creating" {
		logger.Warnf("Instance %s id being created, skipping for now", name)
		return
	}

	if instance.RDSInstance.Endpoint == nil {
		logger.Errorf("Instance %s do not have an endpoint yet, the instance is in state: %s", name, *instance.RDSInstance.DBInstanceStatus)
		return
	}

	addr := aws.ToString(instance.RDSInstance.Endpoint.Address)
	port := aws.ToInt64(aws.Int64(int64(*instance.RDSInstance.Endpoint.Port)))

	isSlave := instance.RDSInstance.ReadReplicaSourceDBInstanceIdentifier != nil
	isMaster := len(instance.RDSInstance.ReadReplicaDBInstanceIdentifiers) > 0

	logger.Debugf("  ID:   %s", id)
	logger.Debugf("  Name: %s", name)
	logger.Debugf("  Addr: %s", addr)
	logger.Debugf("  Port: %d", port)

	tags := make([]string, 0)
	if isSlave {
		tags = append(tags, r.consulReplicaTag)
		id = fmt.Sprintf("%s-%s-%s", id, *instance.RDSInstance.DBInstanceIdentifier, r.consulReplicaTag)
	}

	if isMaster {
		tags = append(tags, r.consulMasterTag)
		id = id + "-" + r.consulMasterTag
	}

	if !isSlave && !isMaster {
		tags = append(tags, r.consulMasterTag)
		tags = append(tags, r.consulReplicaTag)
	}

	status := "passing"
	switch aws.ToString(instance.RDSInstance.DBInstanceStatus) {
	case "backing-up":
		status = "passing"
	case "available":
		status = "passing"
	case "maintenance":
		status = "passing"
	case "modifying":
		status = "passing"
	case "creating":
		status = "critical"
	case "deleting":
		status = "critical"
	case "failed":
		status = "critical"
	case "rebooting":
		status = "passing"
	case "renaming":
		status = "critical"
	case "restore-error":
		status = "critical"
	case "inaccessible-encryption-credentials":
		status = "critical"
	case "incompatible-credentials":
		status = "critical"
	case "incompatible-network":
		status = "critical"
	case "incompatible-option-group":
		status = "critical"
	case "incompatible-parameters":
		status = "critical"
	case "incompatible-restore":
		status = "critical"
	case "resetting-master-credentials":
		status = "warning"
	case "storage-optimization":
		status = "passing"
	case "storage-full":
		status = "warning"
	case "upgrading":
		status = "warning"
	default:
		status = "passing"
	}

	service := &config.Service{
		ServiceID:      id,
		ServiceName:    name,
		ServiceAddress: addr,
		ServicePort:    int(port),
		ServiceTags:    tags,
		CheckID:        fmt.Sprintf("service:%s", id),
		CheckNode:      r.consulNodeName,
		CheckNotes:     fmt.Sprintf("RDS Instance Status: %s", aws.ToString(instance.RDSInstance.DBInstanceStatus)),
		CheckStatus:    status,
		CheckOutput:    fmt.Sprintf("Pending tasks: %+v\n\nAddr: %s\n\nmanaged by aws-dynamic-consul-catalog", instance.RDSInstance.PendingModifiedValues, addr),
	}

	service.ServiceMeta = make(map[string]string)
	service.ServiceMeta["Engine"] = aws.ToString(instance.RDSInstance.Engine)
	service.ServiceMeta["EngineVersion"] = aws.ToString(instance.RDSInstance.EngineVersion)
	service.ServiceMeta["DBName"] = aws.ToString(instance.RDSInstance.DBName)
	service.ServiceMeta["DBInstanceClass"] = aws.ToString(instance.RDSInstance.DBInstanceClass)
	service.ServiceMeta["DBInstanceIdentifier"] = aws.ToString(instance.RDSInstance.DBInstanceIdentifier)
	service.ServiceMeta["DBClusterIdentifier"] = aws.ToString(instance.RDSInstance.DBClusterIdentifier)

	if stringInSlice(service.ServiceID, seen.Services) {
		logger.Errorf("Found duplicate Service ID %s - possible duplicate 'consul_service_name' RDS tag with same Replication Role", service.ServiceID)
		if r.onDuplicate == "quit" {
			os.Exit(1)
		}
		if r.onDuplicate == "ignore-skip-last" {
			logger.Errorf("Ignoring current service")
			return
		}
	}
	seen.Services = append(seen.Services, service.ServiceID)

	if stringInSlice(service.CheckID, seen.Checks) {
		logger.Errorf("Found duplicate Check ID %s - possible duplicate 'consul_service_name' RDS tag with same Replication Role", service.CheckID)
		if r.onDuplicate == "quit" {
			os.Exit(1)
		}
		if r.onDuplicate == "ignore-skip-last" {
			logger.Errorf("Ignoring current service")
			return
		}
	}
	seen.Checks = append(seen.Checks, service.CheckID)

	existingService, ok := state.Services[id]
	if ok {
		logger.Debugf("Service %s exist in remote catalog, lets compare", id)

		if r.identicalService(existingService, service, logger) {
			logger.Debugf("Services are identical, skipping")
			return
		}

		logger.Info("Services are not identical, updating catalog")
	} else {
		logger.Infof("Service %s doesn't exist in remote catalog, creating", id)
	}

	service.CheckOutput = service.CheckOutput + fmt.Sprintf("\n\nLast update: %s", time.Now().Format(time.RFC1123Z))
	r.backend.WriteService(service)
}

func (r *RDS) getServiceName(instance *config.RDSInstances) string {
	logger := log.WithField("rds", "getServiceName")
	logger.Info("Starting RDS Consul getServiceName")
	// prefer the consul_service_name from instance tags
	if name, ok := instance.Tags["consul_service_name"]; ok {
		return r.servicePrefix + name + r.serviceSuffix
	}

	// derive from the instance DB name
	var name string
	if name != "" {
		return r.servicePrefix + name + r.serviceSuffix
	}

	log.Errorf("Failed to find service name for " + aws.ToString(instance.RDSInstance.DBInstanceArn))
	return ""
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
