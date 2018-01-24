package rds

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	observer "github.com/imkira/go-observer"
	cache "github.com/patrickmn/go-cache"
	"github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

func (r *RDS) reader(prop observer.Property) {
	logger := log.WithField("worker", "indexer")
	logger.Info("Starting RDS index worker")

	ticker := time.NewTicker(r.checkInterval)

	// read right away on start
	r.read(prop, logger)

	for {
		select {
		case <-r.quitCh:
			return
		case <-ticker.C:
			r.read(prop, logger)
		}
	}
}

func (r *RDS) read(prop observer.Property, logger *log.Entry) {
	logger.Debug("Starting refresh of RDS information")
	resp, err := r.rds.DescribeDBInstances(&rds.DescribeDBInstancesInput{})
	if err != nil {
		log.Fatal(err)
	}

	instances := make([]*config.DBInstance, 0)
	for _, instance := range resp.DBInstances {
		instances = append(instances, &config.DBInstance{instance, r.getInstanceTags(instance)})
	}

	prop.Update(instances)
	logger.Debug("Finished refresh of RDS information")
}

func (r *RDS) getInstanceTags(instance *rds.DBInstance) config.Tags {
	instanceArn := aws.StringValue(instance.DBInstanceArn)

	cachedTags, found := r.tagCache.Get(instanceArn)
	if found {
		log.Debugf("Found tags in cache for %s", instanceArn)
		return *cachedTags.(*config.Tags)
	}

	input := &rds.ListTagsForResourceInput{ResourceName: instance.DBInstanceArn}
	x, err := r.rds.ListTagsForResource(input)
	if err != nil {
		log.Fatal(err)
	}

	res := make(config.Tags)

	for _, tag := range x.TagList {
		res[*tag.Key] = *tag.Value
	}

	r.tagCache.Set(instanceArn, &res, cache.DefaultExpiration)

	return res
}
