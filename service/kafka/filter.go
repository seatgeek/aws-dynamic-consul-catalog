package kafka

import (
	"strings"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	observer "github.com/imkira/go-observer"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

func (r *KAFKA) filter(all, filtered observer.Property) {
	logger := log.WithField("kafka", "filter")
	logger.Debug("Starting KAFKA instance filter worker")

	stream := all.Observe()

	for {
		select {
		case <-r.quitCh:
			return

		// wait for changes
		case <-stream.Changes():
			logger.Debug("Starting filtering KAFKA instances")

			stream.Next()
			instances := stream.Value().([]*config.MSKCluster)

			filteredInstances := make([]*config.MSKCluster, 0)

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
			logger.Debug("Finished filtering KAFKA instances")
		}
	}
}

func (r *KAFKA) filterByInstanceData(instance *config.MSKCluster, filters config.Filters) bool {
	if len(filters) == 0 {
		return true
	}

	for k, filter := range filters {
		switch k {
		case "ClusterArn":
			return r.matches(filter, aws.ToString(instance.ClusterArn))
		case "ClusterName":
			return r.startsWith(filter, aws.ToString(instance.ClusterName))
		default:
			log.Fatalf("Unknown instance filter key %s (%s)", k, filter)
		}
	}

	return true
}

func (r *KAFKA) matches(filter, value string) bool {
	for _, v := range strings.Split(filter, ",") {
		if v == value {
			return true
		}
	}

	return false
}

func (r *KAFKA) startsWith(filter, value string) bool {
	for _, v := range strings.Split(filter, ",") {
		if strings.HasPrefix(value, v) {
			return true
		}
	}
	return false
}

func (r *KAFKA) filterByInstanceTags(instance *config.MSKCluster, filters config.Filters) bool { // @TODO: filter by tags
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
