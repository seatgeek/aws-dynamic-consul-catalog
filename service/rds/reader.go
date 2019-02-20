package rds

import (
	"os"
	"os/signal"
	"syscall"
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

	ticker := time.NewTimer(r.checkInterval)

	// signal handler
	// sending a SIGUSR1 will trigger a read right away,
	// postponing any scheduled runs
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGUSR1)

	// read right away on start
	r.read(prop, logger)

	for {
		select {
		case <-r.quitCh:
			return

		case <-sigs:
			r.read(prop, logger)          // run updater
			ticker.Reset(r.checkInterval) // schedule new timed run

		case <-ticker.C:
			r.read(prop, logger)          // run updater
			ticker.Reset(r.checkInterval) // schedule new timed run
		}
	}
}

func (r *RDS) read(prop observer.Property, logger *log.Entry) {
	logger.Debug("Starting refresh of RDS information")

	var marker *string
	pages := 0
	instances := make([]*config.DBInstance, 0)
	errorCount := 0

	for {
		pages = pages + 1
		if marker != nil {
			logger.Debugf("Reading RDS information page %d (from marker: %s)", pages, *marker)
		} else {
			logger.Debug("Reading RDS information page 1")
		}

		resp, err := r.rds.DescribeDBInstances(&rds.DescribeDBInstancesInput{
			Marker:     marker,
			MaxRecords: aws.Int64(100),
		})
		if err != nil {
			logger.Errorf("Could not read RDS instances: %+v", err)
			time.Sleep(5 * time.Second)
			errorCount = errorCount + 1

			if errorCount >= 10 {
				log.Fatal("Could not get RDS instances after 10 retries")
			}

			continue
		}
		errorCount = 0

		marker = resp.Marker
		for _, instance := range resp.DBInstances {
			instances = append(instances, &config.DBInstance{instance, r.getInstanceTags(instance)})
		}

		if marker == nil {
			logger.Debugf("Finished reading RDS information page (saw %d pages)", pages)
			break
		}
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
