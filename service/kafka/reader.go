package kafka

import (
	"context"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	kafka "github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	observer "github.com/imkira/go-observer"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	log "github.com/sirupsen/logrus"
)

func (r *KAFKA) reader(prop observer.Property) {
	logger := log.WithField("kafka", "reader")
	logger.Info("Starting KAFKA index worker")

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

func (r *KAFKA) read(prop observer.Property, logger *log.Entry) {
	logger.Debug("Starting refresh of KAFKA information")

	var marker *string
	pages := 0
	instances := make([]*config.MSKCluster, 0)
	errorCount := 0

	for {
		pages = pages + 1
		if marker != nil {
			logger.Debugf("Reading KAFKA information page %d (from marker: %s)", pages, *marker)
		} else {
			logger.Debug("Reading KAFKA information page 1")
		}

		resp, err := r.kafka.ListClustersV2(context.TODO(), &kafka.ListClustersV2Input{
			NextToken:  marker,
			MaxResults: aws.Int32(100),
		})
		if err != nil {
			logger.Debugf("Using AWS ARN %s", os.Getenv("AWS_ROLE_ARN"))
			logger.Errorf("Could not read KAFKA instances: %+v", err)
			time.Sleep(5 * time.Second)
			errorCount = errorCount + 1

			if errorCount >= 10 {
				log.Fatal("Could not get KAFKA instances after 10 retries")
			}

			continue
		}
		errorCount = 0

		marker = resp.NextToken
		for _, instance := range resp.ClusterInfoList {
			instances = append(instances, &config.MSKCluster{
				Cluster: &instance,
				Tags:    instance.Tags,
				Brokers: r.getBrokers(&instance),
			})
		}

		if marker == nil {
			logger.Debugf("Finished reading KAFKA information page (saw %d pages)", pages)
			break
		}
	}

	prop.Update(instances)
	logger.Debug("Finished refresh of KAFKA information")
}

func (r *KAFKA) getBrokers(instance *kafkatypes.Cluster) []config.Brokers {
	clusterArn := aws.ToString(instance.ClusterArn)

	input := &kafka.GetBootstrapBrokersInput{
		ClusterArn: aws.String(clusterArn),
	}

	result, err := r.kafka.GetBootstrapBrokers(context.TODO(), input)
	if err != nil {
		log.Fatal(err)
	}

	res := []config.Brokers{}

	log.Infof("Adding brokers for cluster %s", clusterArn)
	if result != nil {
		log.Infof("BootstrapBrokerStringSaslScram: %s", aws.ToString(result.BootstrapBrokerStringSaslScram))
		brokers := aws.ToString(result.BootstrapBrokerStringSaslScram)
		for _, broker := range strings.Split(brokers, ",") {
			res = append(res, config.Brokers{
				Host: strings.Split(broker, ":")[0],
				Port: func() int {
					port, err := strconv.Atoi(strings.Split(broker, ":")[1])
					if err != nil {
						log.Fatalf("Invalid port number: %v", err)
					}
					return port
				}(),
			})
			log.Infof("Adding broker %s", broker)
		}
	}

	return res
}
