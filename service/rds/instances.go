package rds

import (
	"fmt"
	"os"
	"time"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

func (r *RDS) writeBackendCatalogInstances(instance *config.RDSInstances, logger *log.Entry, state *config.CatalogState, seen *config.SeenCatalog) {
	logger.Debug("Starting RDS Consul writeBackendCatalog")

	name := aws.ToString(instance.RDSInstance.DBInstanceIdentifier)
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

	isMaster := instance.RDSInstance.ReadReplicaDBInstanceIdentifiers != nil
	isSlave := len(instance.RDSInstance.ReadReplicaDBInstanceIdentifiers) > 0

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
	case "backing-up", "available", "maintenance", "modifying", "rebooting", "storage-optimization":
		status = "passing"
	case "creating", "deleting", "failed", "renaming", "restore-error", "inaccessible-encryption-credentials", "incompatible-credentials", "incompatible-network", "incompatible-option-group", "incompatible-parameters", "incompatible-restore":
		status = "critical"
	case "resetting-master-credentials", "upgrading", "storage-full":
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
