package rds

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	rds "github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	observer "github.com/imkira/go-observer"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

func (r *RDS) reader(prop observer.Property) {
	logger := log.WithField("rds", "reader")
	logger.Debug("Starting RDS reader")

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

	instances := make([]*config.RDSInstances, 0)
	// clusters := make([]*config.RDSClusters, 0)
	// globalclusters := make([]*config.RDSGlobalCluster, 0)

	res, err := r.fetchRDSResources()
	if err != nil {
		log.Fatalf("Failed to fetch RDS resources: %v", err)
	}

	for _, instance := range res.DBInstances {
		logger.Debugf("Found RDS instance: %s", aws.ToString(instance.DBInstanceArn))
		instances = append(instances, &config.RDSInstances{
			RDSInstance: &instance,
			Tags:        convertTags(instance.TagList),
		})
	}
	// for _, cluster := range res.DBClusters {
	// 	logger.Debugf("Found RDS Cluster: %s", aws.ToString(cluster.DBClusterArn))
	// 	clusters = append(clusters, &config.RDSClusters{
	// 		RDSCluster: &cluster,
	// 		Tags:       convertTags(cluster.TagList),
	// 	})
	// }
	// for _, globalcluster := range res.GlobalClusters {
	// 	logger.Debugf("Found RDS GlobalClusters: %s", aws.ToString(globalcluster.GlobalClusterArn))
	// 	globalclusters = append(globalclusters, &config.RDSGlobalCluster{
	// 		RDSGlobalCluster: &globalcluster,
	// 		Tags:             convertTags(globalcluster.TagList),
	// 	})
	// }

	prop.Update(instances)
	// prop.Update(clusters)
	// prop.Update(globalclusters)
	logger.Debug("Finished refresh of RDS information")
}

func (r *RDS) fetchRDSResources() (*config.RDSResources, error) {
	logger := log.WithField("rds", "fetchRDSResources")
	logger.Debug("Starting RDS fetchRDSResources")
	resources := &config.RDSResources{}
	var marker *string
	pages := 0
	errorCount := 0

	// for {
	pages = 0
	errorCount = 0
	marker = nil
	for {
		pages = pages + 1
		if marker != nil {
			logger.Debugf("Reading RDS instances information page %d (from marker: %s)", pages, *marker)
		} else {
			logger.Debugf("Reading RDS instances information page %d", pages)
		}

		dbInstancesOutput, err := r.rds.DescribeDBInstances(context.TODO(), &rds.DescribeDBInstancesInput{
			Marker:     marker,
			MaxRecords: aws.Int32(100),
		})
		if err != nil {
			log.Errorf("Failed to describe DB instances: %v", err)
			time.Sleep(5 * time.Second)
			errorCount = errorCount + 1
		}
		if errorCount >= 10 {
			log.Fatal("Could not get RDS instances after 10 retries")
		}
		resources.DBInstances = append(resources.DBInstances, dbInstancesOutput.DBInstances...)
		marker = dbInstancesOutput.Marker
		if marker == nil {
			break
		}
	}
	// pages = 0
	// errorCount = 0
	// marker = nil
	// for {
	// 	pages = pages + 1
	// 	if marker != nil {
	// 		logger.Debugf("Reading RDS clusters information page %d (from marker: %s)", pages, *marker)
	// 	} else {
	// 		logger.Debugf("Reading RDS clusters information page %d", pages)
	// 	}

	// 	dbClustersOutput, err := r.rds.DescribeDBClusters(context.TODO(), &rds.DescribeDBClustersInput{
	// 		Marker:     marker,
	// 		MaxRecords: aws.Int32(100),
	// 	})
	// 	if err != nil {
	// 		log.Errorf("Failed to describe DB clusters: %v", err)
	// 		time.Sleep(5 * time.Second)
	// 		errorCount = errorCount + 1
	// 	}
	// 	if errorCount >= 10 {
	// 		log.Fatal("Could not get RDS clusters after 10 retries")
	// 	}
	// 	resources.DBClusters = append(resources.DBClusters, dbClustersOutput.DBClusters...)
	// 	marker = dbClustersOutput.Marker
	// 	if marker == nil {
	// 		break
	// 	}
	// }
	// pages = 0
	// errorCount = 0
	// marker = nil
	// for {
	// 	pages = pages + 1
	// 	if marker != nil {
	// 		logger.Debugf("Reading RDS global clusters information page %d (from marker: %s)", pages, *marker)
	// 	} else {
	// 		logger.Debugf("Reading RDS global clusters information page %d", pages)
	// 	}

	// 	globalClustersOutput, err := r.rds.DescribeGlobalClusters(context.TODO(), &rds.DescribeGlobalClustersInput{
	// 		Marker:     marker,
	// 		MaxRecords: aws.Int32(100),
	// 	})
	// 	if err != nil {
	// 		log.Errorf("Failed to describe global clusters: %v", err)
	// 		time.Sleep(5 * time.Second)
	// 		errorCount = errorCount + 1
	// 	}
	// 	if errorCount >= 10 {
	// 		log.Fatal("Could not get RDS instances after 10 retries")
	// 	}
	// 	resources.GlobalClusters = append(resources.GlobalClusters, globalClustersOutput.GlobalClusters...)
	// 	marker = globalClustersOutput.Marker
	// 	if marker == nil {
	// 		break
	// 	}
	// }
	return resources, nil
}

func convertTags(tagList []rdstypes.Tag) config.Tags {
	tags := make(config.Tags)
	for _, tag := range tagList {
		tags[*tag.Key] = *tag.Value
	}
	return tags
}
