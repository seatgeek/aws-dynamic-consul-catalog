package elasticache

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"time"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	elasticache "github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	observer "github.com/imkira/go-observer"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

var removeUpdatedTimeRegexp = regexp.MustCompile("\n\nLast update: .+")

func (r *ELASTICACHE) writer(prop observer.Property, state *config.CatalogState) {
	logger := log.WithField("elasticache", "writer")
	logger.Debug("Starting ELASTICACHE Consul Catalog writer")

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
			instances := stream.Value().([]*config.Elasticache)

			seen := state.Services.GetSeen()

			found := &config.SeenCatalog{
				Services: make([]string, 0),
				Checks:   make([]string, 0),
			}

			for _, instance := range instances {
				instance.ReplicationGroup = func() []*elasticachetypes.ReplicationGroup {
					nodeRole, err := r.getNodeRole(aws.ToString(instance.CacheCluster.ReplicationGroupId))
					if err != nil {
						logger.Errorf("Failed to get node role for instance %s: %v", aws.ToString(instance.CacheCluster.ReplicationGroupId), err)
						return nil
					}
					return nodeRole
				}()
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

func (r *ELASTICACHE) writeBackendCatalog(instance *config.Elasticache, logger *log.Entry, state *config.CatalogState, seen *config.SeenCatalog) {

	logger = logger.WithField("instance", aws.ToString(instance.CacheCluster.CacheClusterId))

	name := aws.ToString(instance.CacheCluster.CacheClusterId)

	if *instance.CacheCluster.CacheClusterStatus == "creating" {
		logger.Warnf("Instance %s is being created, skipping for now", name)
		return
	}

	if instance.CacheCluster.CacheNodes == nil {
		logger.Errorf("Instance %s does not have an endpoint yet, the instance is in state: %v", name, instance.CacheCluster.CacheNodes)
		return
	}

	for i, node := range instance.CacheCluster.CacheNodes {
		addr := node.Endpoint.Address
		port := node.Endpoint.Port

		iscluster := instance.ReplicationGroup[0].ClusterEnabled
		isPrimary := aws.ToString(instance.ReplicationGroup[0].NodeGroups[0].NodeGroupMembers[int(name[len(name)-1]-'0')-1].CurrentRole) == "primary" && !*iscluster
		isReplica := aws.ToString(instance.ReplicationGroup[0].NodeGroups[0].NodeGroupMembers[int(name[len(name)-1]-'0')-1].CurrentRole) == "replica" && !*iscluster

		serviceID := fmt.Sprintf("%s-%d", name, i)
		name = instance.Tags["stack"] + "-" + instance.Tags["environment"] + "-" + instance.Tags["name"]

		tags := make([]string, 0)
		if isReplica {
			tags = append(tags, r.consulReplicaTag)
			serviceID = serviceID + "-" + r.consulReplicaTag
		}

		if isPrimary {
			tags = append(tags, r.consulPrimaryTag)
			serviceID = serviceID + "-" + r.consulPrimaryTag
		}

		if *iscluster {
			tags = append(tags, r.consulClusterTag)
			serviceID = serviceID + "-" + r.consulClusterTag
		}

		status := "passing"
		switch aws.ToString(instance.CacheCluster.CacheClusterStatus) {
		case "modifying", "cluster nodes", "snapshotting":
			status = "passing"
		case "creating", "rebooting", "deleting", "deleted", "restore-failed ":
			status = "critical"
		case "incompatible-network":
			status = "warning"
		default:
			status = "passing"
		}

		service := &config.Service{
			ServiceID:      serviceID,
			ServiceName:    name,
			ServiceAddress: *addr,
			ServicePort:    int(*port),
			ServiceTags:    tags,
			CheckID:        fmt.Sprintf("service:%s", serviceID),
			CheckNode:      r.consulNodeName,
			CheckNotes:     fmt.Sprintf("ELASTICACHE Instance Status: %s", aws.ToString(instance.CacheCluster.CacheClusterId)),
			CheckStatus:    status,
			CheckOutput:    fmt.Sprintf("Pending tasks: %s\n\n\nmanaged by aws-dynamic-consul-catalog", aws.ToString(instance.CacheCluster.CacheClusterId)),
		}

		service.ServiceMeta = make(map[string]string)
		service.ServiceMeta["ClusterName"] = aws.ToString(instance.CacheCluster.CacheClusterId)

		if stringInSlice(service.ServiceID, seen.Services) {
			logger.Errorf("Found duplicate Service ID %s - possible duplicate 'consul_service_name' ELASTICACHE tag with same Replication Role", service.ServiceID)
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
			logger.Errorf("Found duplicate Check ID %s - possible duplicate 'consul_service_name' ELASTICACHE tag with same Replication Role", service.CheckID)
			if r.onDuplicate == "quit" {
				os.Exit(1)
			}
			if r.onDuplicate == "ignore-skip-last" {
				logger.Errorf("Ignoring current service")
				return
			}
		}
		seen.Checks = append(seen.Checks, service.CheckID)

		existingService, ok := state.Services[serviceID]
		if ok {
			logger.Debugf("Service %s exists in remote catalog, let's compare", serviceID)

			if r.identicalService(existingService, service, logger) {
				logger.Debugf("Services are identical, skipping")
				continue
			}

			logger.Info("Services are not identical, updating catalog")
		} else {
			logger.Infof("Service %s doesn't exist in remote catalog, creating", serviceID)
		}

		service.CheckOutput = service.CheckOutput + fmt.Sprintf("\n\nLast update: %s", time.Now().Format(time.RFC1123Z))
		r.backend.WriteService(service)
	}
}

func (r *ELASTICACHE) identicalService(a, b *config.Service, logger *log.Entry) bool {
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

func (r *ELASTICACHE) getDifference(slice1, slice2 []string) []string {
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

func (r *ELASTICACHE) isDifferent(slice1, slice2 []string) bool {
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

func (r *ELASTICACHE) getNodeRole(replicationGroupId string) ([]*elasticachetypes.ReplicationGroup, error) {
	log.Debugf("Getting node role for replication group %s", replicationGroupId)
	input := &elasticache.DescribeReplicationGroupsInput{
		ReplicationGroupId: &replicationGroupId,
	}

	resp, err := r.elasticache.DescribeReplicationGroups(context.TODO(), input)
	if err != nil {
		log.Printf("Failed to list tags for resource %s: %v", replicationGroupId, err)
		return nil, err
	}

	if len(resp.ReplicationGroups) > 0 {
		replicationGroups := make([]*elasticachetypes.ReplicationGroup, len(resp.ReplicationGroups))
		for i := range resp.ReplicationGroups {
			replicationGroups[i] = &resp.ReplicationGroups[i]
		}
		return replicationGroups, nil
	}
	return nil, nil
}
