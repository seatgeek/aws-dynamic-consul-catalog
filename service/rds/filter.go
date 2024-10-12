package rds

import (
	"strings"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	observer "github.com/imkira/go-observer"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

func (r *RDS) filter(all, filtered observer.Property) {
	logger := log.WithField("rds", "filter")
	logger.Debug("Starting RDS instance filter worker")
	stream := all.Observe()

	for {
		select {
		case <-r.quitCh:
			return

		// wait for changes
		case <-stream.Changes():
			logger.Debug("Starting filtering RDS instances")

			for stream.HasNext() {
				stream.Next()
				switch v := stream.Value().(type) {
				case []*config.RDSInstances:
					instances := stream.Value().([]*config.RDSInstances)
					filteredInstances := make([]*config.RDSInstances, 0)
					logger.Debugf("Filtering %d RDS instances", len(instances))
					logger.Debugf("Instance filters: %v", r.instanceFilters)
					logger.Debugf("Tag filters: %v", r.tagFilters)
					for _, instance := range instances {
						if !r.filterByInstanceData(instance, r.instanceFilters) {
							continue
						}

						if !r.filterByInstanceTags(instance, r.tagFilters) {
							continue
						}

						filteredInstances = append(filteredInstances, instance)
					}
					filtered.Update(filteredInstances)
				// case []*config.RDSClusters:
				// 	clusters := stream.Value().([]*config.RDSClusters)
				// 	fileteredClusters := make([]*config.RDSClusters, 0)
				// 	logger.Debugf("Filtering %d RDS clusters", len(clusters))
				// 	logger.Debugf("Instance filters: %v", r.instanceFilters)
				// 	logger.Debugf("Tag filters: %v", r.tagFilters)
				// 	for _, cluster := range clusters {
				// 		if !r.filterByClusterData(cluster, r.instanceFilters) {
				// 			continue
				// 		}
				// 		if !r.filterByClusterTags(cluster, r.tagFilters) {
				// 			continue
				// 		}
				// 		fileteredClusters = append(fileteredClusters, cluster)
				// 	}
				// 	filtered.Update(fileteredClusters)
				// case []*config.RDSGlobalCluster:
				// 	globalclusters := stream.Value().([]*config.RDSGlobalCluster)
				// 	filteredGlobalClusters := make([]*config.RDSGlobalCluster, 0)
				// 	logger.Debugf("Filtering %d RDS global clusters", len(globalclusters))
				// 	logger.Debugf("Instance filters: %v", r.instanceFilters)
				// 	logger.Debugf("Tag filters: %v", r.tagFilters)
				// 	for _, globalcluster := range globalclusters {
				// 		if !r.filterByGlobalClusterData(globalcluster, r.instanceFilters) {
				// 			continue
				// 		}
				// 		if !r.filterByGlobalClusterTags(globalcluster, r.tagFilters) {
				// 			continue
				// 		}
				// 		filteredGlobalClusters = append(filteredGlobalClusters, globalcluster)
				// 	}
				// 	filtered.Update(filteredGlobalClusters)
				// 	logger.Debug("Finished filtering RDS instances")
				default:
					log.Printf("Unexpected type %T", v)
				}
			}
		}
	}
}

func (r *RDS) filterByInstanceData(instance *config.RDSInstances, filters config.Filters) bool {
	if len(filters) == 0 {
		return true
	}

	for k, filter := range filters {
		switch k {
		case "DBInstanceClass":
			return r.matches(filter, aws.ToString(instance.RDSInstance.DBInstanceClass))
		case "DBInstanceIdentifier":
			return r.matches(filter, aws.ToString(instance.RDSInstance.DBInstanceIdentifier))
		case "DBClusterIdentifier":
			return r.matches(filter, aws.ToString(instance.RDSInstance.DBClusterIdentifier))
		case "DBInstanceStatus":
			return r.matches(filter, aws.ToString(instance.RDSInstance.DBInstanceStatus))
		case "Engine":
			return r.matches(filter, aws.ToString(instance.RDSInstance.Engine))
		default:
			log.Fatalf("Unknown instance filter key %s (%s)", k, filter)
		}
	}

	return true
}

// func (r *RDS) filterByClusterData(cluster *config.RDSClusters, filters config.Filters) bool {
// 	if len(filters) == 0 {
// 		return true
// 	}

// 	for k, filter := range filters {
// 		switch k {
// 		case "DBClusterInstanceClass":
// 			return r.matches(filter, aws.ToString(cluster.RDSCluster.DBClusterInstanceClass))
// 		case "DBClusterIdentifier":
// 			return r.matches(filter, aws.ToString(cluster.RDSCluster.DBClusterIdentifier))
// 		case "Engine":
// 			return r.matches(filter, aws.ToString(cluster.RDSCluster.Engine))
// 		default:
// 			log.Fatalf("Unknown instance filter key %s (%s)", k, filter)
// 		}
// 	}

// 	return true
// }

// func (r *RDS) filterByGlobalClusterData(globalcluster *config.RDSGlobalCluster, filters config.Filters) bool {
// 	if len(filters) == 0 {
// 		return true
// 	}

// 	for k, filter := range filters {
// 		switch k {
// 		case "GlobalClusterIdentifier":
// 			return r.matches(filter, aws.ToString(globalcluster.RDSGlobalCluster.GlobalClusterIdentifier))
// 		case "Status":
// 			return r.matches(filter, aws.ToString(globalcluster.RDSGlobalCluster.Status))
// 		case "Engine":
// 			return r.matches(filter, aws.ToString(globalcluster.RDSGlobalCluster.Engine))
// 		case "EngineVersion":
// 			return r.matches(filter, aws.ToString(globalcluster.RDSGlobalCluster.EngineVersion))
// 		default:
// 			log.Fatalf("Unknown globalcluster filter key %s (%s)", k, filter)
// 		}
// 	}

// 	return true
// }

func (r *RDS) matches(filter, value string) bool {
	for _, v := range strings.Split(filter, ",") {
		if v == value {
			return true
		}
	}

	return false
}

func (r *RDS) filterByInstanceTags(instance *config.RDSInstances, filters config.Filters) bool {
	if len(filters) == 0 {
		return true
	}

	tags := instance.Tags

	for k, v := range filters {
		val, ok := tags[k]

		// the tag key doesn't exist
		if !ok {
			return false
		}

		// the value doesn't match
		if val != v {
			return false
		}
	}

	return true
}

// func (r *RDS) filterByClusterTags(cluster *config.RDSClusters, filters config.Filters) bool {
// 	if len(filters) == 0 {
// 		return true
// 	}

// 	tags := cluster.Tags

// 	for k, v := range filters {
// 		val, ok := tags[k]

// 		// the tag key doesn't exist
// 		if !ok {
// 			return false
// 		}

// 		// the value doesn't match
// 		if val != v {
// 			return false
// 		}
// 	}

// 	return true
// }

// func (r *RDS) filterByGlobalClusterTags(globalcluster *config.RDSGlobalCluster, filters config.Filters) bool {
// 	if len(filters) == 0 {
// 		return true
// 	}

// 	tags := globalcluster.Tags

// 	for k, v := range filters {
// 		val, ok := tags[k]

// 		// the tag key doesn't exist
// 		if !ok {
// 			return false
// 		}

// 		// the value doesn't match
// 		if val != v {
// 			return false
// 		}
// 	}

// 	return true
// }
