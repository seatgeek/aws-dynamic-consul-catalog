package elasticache

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	elasticache "github.com/aws/aws-sdk-go-v2/service/elasticache"
	observer "github.com/imkira/go-observer"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

func (r *ELASTICACHE) reader(prop observer.Property) {
	logger := log.WithField("elasticache", "reader")
	logger.Debug("Starting ELASTICACHE index worker")

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

func (r *ELASTICACHE) read(prop observer.Property, logger *log.Entry) {
	logger.Debug("Starting refresh of ELASTICACHE information")

	var marker *string
	pages := 0
	instances := make([]*config.Elasticache, 0)
	errorCount := 0

	for {
		pages = pages + 1
		if marker != nil {
			logger.Debugf("Reading ELASTICACHE information page %d (from marker: %s)", pages, *marker)
		} else {
			logger.Debug("Reading ELASTICACHE information page 1")
		}

		resp, err := r.elasticache.DescribeCacheClusters(context.TODO(), &elasticache.DescribeCacheClustersInput{
			Marker:            marker,
			MaxRecords:        aws.Int32(100),
			ShowCacheNodeInfo: aws.Bool(true),
		})
		if err != nil {
			logger.Debugf("Using AWS ARN %s", os.Getenv("AWS_ROLE_ARN"))
			logger.Errorf("Could not read ELASTICACHE instances: %+v", err)
			time.Sleep(5 * time.Second)
			errorCount = errorCount + 1

			if errorCount >= 10 {
				log.Fatal("Could not get ELASTICACHE instances after 10 retries")
			}

			continue
		}
		errorCount = 0

		marker = resp.Marker
		for _, instance := range resp.CacheClusters {
			instances = append(instances, &config.Elasticache{
				CacheCluster: &instance,
				Tags: func() config.Tags {
					tags, err := r.getElastiCacheTags(aws.ToString(instance.ARN))
					if err != nil {
						logger.Errorf("Failed to get tags for instance %s: %v", aws.ToString(instance.ARN), err)
						return nil
					}
					return tags
				}(),
			})
		}

		if marker == nil {
			logger.Debugf("Finished reading ELASTICACHE information page (saw %d pages)", pages)
			break
		}
	}

	prop.Update(instances)
	logger.Debug("Finished refresh of ELASTICACHE information")
}

func (r *ELASTICACHE) getElastiCacheTags(resourceArn string) (config.Tags, error) {
	input := &elasticache.ListTagsForResourceInput{
		ResourceName: &resourceArn,
	}

	resp, err := r.elasticache.ListTagsForResource(context.TODO(), input)
	if err != nil {
		log.Printf("Failed to list tags for resource %s: %v", resourceArn, err)
		return nil, err
	}

	tags := make(config.Tags)
	for _, tag := range resp.TagList {
		tags[*tag.Key] = *tag.Value
	}

	return tags, nil
}
