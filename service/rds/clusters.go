package rds

// import (
// 	"fmt"
// 	"os"
// 	"time"

// 	aws "github.com/aws/aws-sdk-go-v2/aws"
// 	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
// 	log "github.com/sirupsen/logrus"
// )

// func (r *RDS) writeBackendCatalogClusters(cluster *config.RDSClusters, logger *log.Entry, state *config.CatalogState, seen *config.SeenCatalog) {
// 	logger.Info("Starting RDS Consul writeBackendCatalog")

// 	name := r.getClustersServiceName(cluster)
// 	if name == "" {
// 		return
// 	}
// 	id := name

// 	if *cluster.RDSCluster.Status == "creating" {
// 		logger.Warnf("Instance %s id being created, skipping for now", name)
// 		return
// 	}

// 	if cluster.RDSCluster.Endpoint == nil {
// 		logger.Errorf("Instance %s do not have an endpoint yet, the cluster is in state: %s", name, *cluster.RDSCluster.Status)
// 		return
// 	}

// 	addr := aws.ToString(cluster.RDSCluster.Endpoint)
// 	port := aws.ToInt64(aws.Int64(int64(*cluster.RDSCluster.Port)))

// 	// isSlave := cluster.RDSCluster.ReadReplicaSourceDBInstanceIdentifier != nil
// 	// isMaster := len(cluster.RDSCluster.ReadReplicaDBInstanceIdentifiers) > 0

// 	logger.Debugf("  ID:   %s", id)
// 	logger.Debugf("  Name: %s", name)
// 	logger.Debugf("  Addr: %s", addr)
// 	logger.Debugf("  Port: %d", port)

// 	tags := make([]string, 0)
// 	// if isSlave {
// 	// 	tags = append(tags, r.consulReplicaTag)
// 	// 	id = fmt.Sprintf("%s-%s-%s", id, *cluster.RDSCluster.DBInstanceIdentifier, r.consulReplicaTag)
// 	// }

// 	// if isMaster {
// 	// 	tags = append(tags, r.consulMasterTag)
// 	// 	id = id + "-" + r.consulMasterTag
// 	// }

// 	// if !isSlave && !isMaster {
// 	// 	tags = append(tags, r.consulMasterTag)
// 	// 	tags = append(tags, r.consulReplicaTag)
// 	// }

// 	status := "passing"
// 	switch aws.ToString(cluster.RDSCluster.Status) {
// 	case "backing-up":
// 		status = "passing"
// 	case "available":
// 		status = "passing"
// 	case "maintenance":
// 		status = "passing"
// 	case "modifying":
// 		status = "passing"
// 	case "creating":
// 		status = "critical"
// 	case "deleting":
// 		status = "critical"
// 	case "failed":
// 		status = "critical"
// 	case "rebooting":
// 		status = "passing"
// 	case "renaming":
// 		status = "critical"
// 	case "restore-error":
// 		status = "critical"
// 	case "inaccessible-encryption-credentials":
// 		status = "critical"
// 	case "incompatible-credentials":
// 		status = "critical"
// 	case "incompatible-network":
// 		status = "critical"
// 	case "incompatible-option-group":
// 		status = "critical"
// 	case "incompatible-parameters":
// 		status = "critical"
// 	case "incompatible-restore":
// 		status = "critical"
// 	case "resetting-master-credentials":
// 		status = "warning"
// 	case "storage-optimization":
// 		status = "passing"
// 	case "storage-full":
// 		status = "warning"
// 	case "upgrading":
// 		status = "warning"
// 	default:
// 		status = "passing"
// 	}

// 	service := &config.Service{
// 		ServiceID:      id,
// 		ServiceName:    name,
// 		ServiceAddress: addr,
// 		ServicePort:    int(port),
// 		ServiceTags:    tags,
// 		CheckID:        fmt.Sprintf("service:%s", id),
// 		CheckNode:      r.consulNodeName,
// 		CheckNotes:     fmt.Sprintf("RDS Database Name Status: %s", aws.ToString(cluster.RDSCluster.DatabaseName)),
// 		CheckStatus:    status,
// 		CheckOutput:    fmt.Sprintf("Pending tasks: %+v\n\nAddr: %s\n\nmanaged by aws-dynamic-consul-catalog", cluster.RDSCluster.PendingModifiedValues, addr),
// 	}

// 	service.ServiceMeta = make(map[string]string)
// 	service.ServiceMeta["Engine"] = aws.ToString(cluster.RDSCluster.Engine)
// 	service.ServiceMeta["EngineVersion"] = aws.ToString(cluster.RDSCluster.EngineVersion)
// 	service.ServiceMeta["DatabaseName"] = aws.ToString(cluster.RDSCluster.DatabaseName)
// 	service.ServiceMeta["DBClusterInstanceClass"] = aws.ToString(cluster.RDSCluster.DBClusterInstanceClass)
// 	service.ServiceMeta["DBClusterIdentifier"] = aws.ToString(cluster.RDSCluster.DBClusterIdentifier)

// 	if stringInSlice(service.ServiceID, seen.Services) {
// 		logger.Errorf("Found duplicate Service ID %s - possible duplicate 'consul_service_name' RDS tag with same Replication Role", service.ServiceID)
// 		if r.onDuplicate == "quit" {
// 			os.Exit(1)
// 		}
// 		if r.onDuplicate == "ignore-skip-last" {
// 			logger.Errorf("Ignoring current service")
// 			return
// 		}
// 	}
// 	seen.Services = append(seen.Services, service.ServiceID)

// 	if stringInSlice(service.CheckID, seen.Checks) {
// 		logger.Errorf("Found duplicate Check ID %s - possible duplicate 'consul_service_name' RDS tag with same Replication Role", service.CheckID)
// 		if r.onDuplicate == "quit" {
// 			os.Exit(1)
// 		}
// 		if r.onDuplicate == "ignore-skip-last" {
// 			logger.Errorf("Ignoring current service")
// 			return
// 		}
// 	}
// 	seen.Checks = append(seen.Checks, service.CheckID)

// 	existingService, ok := state.Services[id]
// 	if ok {
// 		logger.Debugf("Service %s exist in remote catalog, lets compare", id)

// 		if r.identicalService(existingService, service, logger) {
// 			logger.Debugf("Services are identical, skipping")
// 			return
// 		}

// 		logger.Info("Services are not identical, updating catalog")
// 	} else {
// 		logger.Infof("Service %s doesn't exist in remote catalog, creating", id)
// 	}

// 	service.CheckOutput = service.CheckOutput + fmt.Sprintf("\n\nLast update: %s", time.Now().Format(time.RFC1123Z))
// 	r.backend.WriteService(service)
// }

// func (r *RDS) getClustersServiceName(cluster *config.RDSClusters) string {
// 	logger := log.WithField("rds", "getClustersServiceName")
// 	logger.Info("Starting RDS Consul getClustersServiceName")
// 	// prefer the consul_service_name from cluster tags
// 	if name, ok := cluster.Tags["consul_service_name"]; ok {
// 		return r.servicePrefix + name + r.serviceSuffix
// 	}

// 	// derive from the cluster DB name
// 	var name string
// 	if name != "" {
// 		return r.servicePrefix + name + r.serviceSuffix
// 	}

// 	log.Errorf("Failed to find cluster service name for " + aws.ToString(cluster.RDSCluster.DBClusterIdentifier))
// 	return aws.ToString(cluster.RDSCluster.DBClusterIdentifier)
// }
