package rds

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/imkira/go-observer"
	"github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

func (r *RDS) clusterWriter(inClustersProp observer.Property, state *config.CatalogState) {
	logger := log.WithField("worker", "clusterWriter")
	logger.Info("Starting RDS Consul Catalog clusterWriter")

	stream := inClustersProp.Observe()
	for {
		select {
		case <-r.quitCh:
			return

		// wait for changes
		case <-stream.Changes():
			state.Lock()
			stream.Next()
			clusters := stream.Value().([]*config.DBCluster)

			seen := state.Services.GetSeen()

			found := &config.SeenCatalog{
				Services: make([]string, 0),
				Checks:   make([]string, 0),
			}

			for _, cluster := range clusters {
				r.writeClusterBackendCatalog(cluster, logger, state, found)
			}

			for _, service := range r.getDifference(seen.Services, found.Services) {
				logger.Warnf("Deleting service %s", service)
				r.backend.DeleteService(service, r.consulNodeName)
			}

			for _, check := range r.getDifference(seen.Checks, found.Checks) {
				logger.Warnf("Deleting check %s", check)
				r.backend.DeleteCheck(check, r.consulNodeName)
			}

			logger.Debug("Finished Consul Catalog cluster write")
			state.Unlock()
		}
	}
}

func (r *RDS) writeClusterBackendCatalog(cluster *config.DBCluster, logger *log.Entry, state *config.CatalogState, seen *config.SeenCatalog) {
	logger = logger.WithField("cluster", aws.StringValue(cluster.DBClusterIdentifier))

	name := r.getClusterServiceName(cluster)
	if name == "" {
		return
	}
	id := aws.StringValue(cluster.DBClusterIdentifier)

	if *cluster.Status == "creating" {
		logger.Warnf("Cluster %s id being created, skipping for now", name)
		return
	}

	if cluster.Endpoint == nil {
		logger.Errorf("Cluster %s does not have an endpoint yet, the cluster is in state: %s",
			name, *cluster.Status)
		return
	}

	clusterStatus := extractStatus(aws.StringValue(cluster.Status))

	memberRole := map[string]string{}
	for _, member := range cluster.DBClusterMembers {
		if aws.BoolValue(member.IsClusterWriter) {
			memberRole[aws.StringValue(member.DBInstanceIdentifier)] = "cluster-writer"
		} else {
			memberRole[aws.StringValue(member.DBInstanceIdentifier)] = "cluster-reader"
		}
	}

	for _, instance := range cluster.Instances {
		tags := []string{}
		for k, v := range instance.Tags {
			tags = append(tags, fmt.Sprintf("%s-%s", k, v))
		}
		tags = append(tags, fmt.Sprintf("cluster-rw-role-%s",
			memberRole[aws.StringValue(instance.DBInstanceIdentifier)]))
		service := &config.Service{
			ServiceID:   id,
			ServiceName: name,
			ServicePort: int(aws.Int64Value(instance.DbInstancePort)),
			ServiceTags: tags,
			CheckID:     fmt.Sprintf("service:%s Node:%s", id, aws.StringValue(instance.DBInstanceIdentifier)),
			CheckNode:   aws.StringValue(instance.DBInstanceIdentifier),
			CheckNotes:  fmt.Sprintf("RDS Instance Status: %s", aws.StringValue(instance.DBInstanceStatus)),
			CheckStatus: clusterStatus,
			CheckOutput: fmt.Sprintf("Pending tasks: %s\n\nAddr: %s\n\nmanaged by aws-dynamic-consul-catalog",
				instance.PendingModifiedValues.GoString(), aws.StringValue(instance.Endpoint.Address)),
			ServiceAddress: aws.StringValue(instance.Endpoint.Address),
		}

		service.ServiceMeta = make(map[string]string)
		service.ServiceMeta["Engine"] = aws.StringValue(instance.Engine)
		service.ServiceMeta["EngineVersion"] = aws.StringValue(instance.EngineVersion)
		service.ServiceMeta["DBName"] = aws.StringValue(instance.DBName)
		service.ServiceMeta["DBInstanceClass"] = aws.StringValue(instance.DBInstanceClass)
		service.ServiceMeta["DBInstanceIdentifier"] = aws.StringValue(instance.DBInstanceIdentifier)
		service.ServiceMeta["RoleInCluster"] = memberRole[aws.StringValue(instance.DBInstanceIdentifier)]
		service.ServiceMeta["ClusterName"] = aws.StringValue(cluster.DBClusterIdentifier)

		if stringInSlice(service.ServiceAddress, seen.Services) {
			logger.Errorf("Found duplicate Service ID %s - possible duplicate 'consul_service_name' RDS tag with same Replication Role", service.ServiceID)
			if r.onDuplicate == "quit" {
				os.Exit(1)
			}
			if r.onDuplicate == "ignore-skip-last" {
				logger.Errorf("Ignoring current service")
				return
			}
		}
		seen.Services = append(seen.Services, service.ServiceAddress)

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
}

func extractStatus(status string) string {
	switch status {
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
	return status
}

func (r *RDS) getClusterServiceName(cluster *config.DBCluster) string {
	// prefer the consul_service_name from instance tags
	if name, ok := cluster.Tags["consul_service_name"]; ok {
		return r.servicePrefix + name + r.serviceSuffix
	}

	// derive from the instance DB cluster name
	name := aws.StringValue(cluster.DBClusterIdentifier)
	if name != "" {
		return r.servicePrefix + name + r.serviceSuffix
	}

	// derive from the instance DB name
	name = aws.StringValue(cluster.DatabaseName)
	if name != "" {
		return r.servicePrefix + name + r.serviceSuffix
	}

	log.Errorf("Failed to find service name for " + aws.StringValue(cluster.DBClusterArn))
	return ""
}
