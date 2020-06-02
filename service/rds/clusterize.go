package rds

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	observer "github.com/imkira/go-observer"
	"github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

func (r *RDS) clusterize(inInstancesProp, outInstancesProp, outClustersProp observer.Property) {
	logger := log.WithField("worker", "clusterize")
	logger.Info("Starting RDS instance clusterize worker")
	stream := inInstancesProp.Observe()

	for {
		select {
		case <-r.quitCh:
			return

		// wait for changes
		case <-stream.Changes():
			logger.Error("#### Clusterize #### Starting clusterizing RDS instances")

			stream.Next()
			instances := stream.Value().([]*config.DBInstance)

			standaloneInstances := make([]*config.DBInstance, 0)
			clusters := make(map[string]*config.DBCluster)

			for _, instance := range instances {
				clusterID := aws.StringValue(instance.DBClusterIdentifier)
				if clusterID == "" {
					standaloneInstances = append(standaloneInstances, instance)
					logger.Debugf("Instance %s is not part of a cluster", aws.StringValue(instance.DBInstanceIdentifier))
					continue
				}

				logger.Infof("Instance %s is part of cluster: %s", aws.StringValue(instance.DBInstanceIdentifier), clusterID)
				cluster, ok := clusters[clusterID]
				if !ok {
					rdsCluster := r.readDBCluster(clusterID)
					if rdsCluster == nil {
						log.Errorf("Error fetching cluster for instance: %s with ClusterID: %s",
							aws.StringValue(instance.DBInstanceIdentifier), clusterID)
						continue
					}

					cluster = &config.DBCluster{
						DBCluster: rdsCluster,
						Instances: []*config.DBInstance{},
						Tags:      make(config.Tags),
					}
					clusters[clusterID] = cluster
				}
				// attach this instance to the cluster...
				cluster.Instances = append(cluster.Instances, instance)
			}

			outInstancesProp.Update(standaloneInstances)

			clusterList := make([]*config.DBCluster, len(clusters))
			i := 0
			for _, v := range clusters {
				clusterList[i] = v
				i++
			}
			outClustersProp.Update(clusterList)
			logger.
				WithField("Worker", "Clusterizer").
				Errorf("Finished clusterizing RDS instances. Clusters: %d Instances: %d",
					len(clusters), len(instances))
		}
	}
}

func (r *RDS) readDBCluster(clusterID string) *rds.DBCluster {
	input := &rds.DescribeDBClustersInput{DBClusterIdentifier: aws.String(clusterID)}
	output, err := r.rds.DescribeDBClusters(input)
	if err != nil {
		log.Fatal(err)
	}
	for _, cluster := range output.DBClusters {
		if clusterID == aws.StringValue(cluster.DBClusterIdentifier) {
			return cluster
		}
	}
	return nil
}
