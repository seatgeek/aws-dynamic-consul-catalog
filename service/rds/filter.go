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
	logger.Info("Starting RDS instance filter worker")
	stream := all.Observe()

	for {
		select {
		case <-r.quitCh:
			return

		// wait for changes
		case <-stream.Changes():
			logger.Debug("Starting filtering RDS instances")

			stream.Next()
			instances := stream.Value().([]*config.DBInstance)

			filteredInstances := make([]*config.DBInstance, 0)

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
			logger.Debug("Finished filtering RDS instances")
		}
	}
}

func (r *RDS) filterByInstanceData(instance *config.DBInstance, filters config.Filters) bool {
	if len(filters) == 0 {
		return true
	}

	for k, filter := range filters {
		switch k {
		case "AvailabilityZone":
			return r.matches(filter, aws.ToString(instance.AvailabilityZone))
		case "DBInstanceArn":
			return r.matches(filter, aws.ToString(instance.DBInstanceArn))
		case "DBInstanceClass":
			return r.matches(filter, aws.ToString(instance.DBInstanceClass))
		case "DBInstanceIdentifier":
			return r.matches(filter, aws.ToString(instance.DBInstanceIdentifier))
		case "DBClusterIdentifier":
			return r.matches(filter, aws.ToString(instance.DBClusterIdentifier))
		case "DBInstanceStatus":
			return r.matches(filter, aws.ToString(instance.DBInstanceStatus))
		case "Engine":
			return r.matches(filter, aws.ToString(instance.Engine))
		case "EngineVersion":
			return r.matches(filter, aws.ToString(instance.EngineVersion))
		case "VpcId":
			return r.matches(filter, aws.ToString(instance.DBSubnetGroup.VpcId))
		default:
			log.Fatalf("Unknown instance filter key %s (%s)", k, filter)
		}
	}

	return true
}

func (r *RDS) matches(filter, value string) bool {
	for _, v := range strings.Split(filter, ",") {
		if v == value {
			return true
		}
	}

	return false
}

func (r *RDS) filterByInstanceTags(instance *config.DBInstance, filters config.Filters) bool {
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
