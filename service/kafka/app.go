package kafka

import (
	"context"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	kafka "github.com/aws/aws-sdk-go-v2/service/kafka"
	observer "github.com/imkira/go-observer"
	cache "github.com/patrickmn/go-cache"
	cc "github.com/seatgeek/aws-dynamic-consul-catalog/backend/consul"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	gelf "github.com/seatgeek/logrus-gelf-formatter"
	log "github.com/sirupsen/logrus"
	cli "gopkg.in/urfave/cli.v1"
)

// KAFKA ...
type KAFKA struct {
	kafka           *kafka.Client
	backend         config.Backend
	instanceFilters config.Filters
	tagFilters      config.Filters
	tagCache        *cache.Cache
	checkInterval   time.Duration
	quitCh          chan int
	onDuplicate     string
	servicePrefix   string
	serviceSuffix   string
	consulNodeName  string
}

// New ...
func New(c *cli.Context) *KAFKA {
	cfg, err := awsconfig.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("failed to load SDK configuration, %v", err)
	}
	logLevel, err := log.ParseLevel(strings.ToUpper(c.GlobalString("log-level")))
	if err != nil {
		log.Fatalf("%s (%s)", err, c.GlobalString("log-level"))
	}
	log.SetLevel(logLevel)

	logFormat := strings.ToLower(c.GlobalString("log-format"))
	switch logFormat {
	case "json":
		log.SetFormatter(new(gelf.GelfFormatter))
	case "text":
		log.SetFormatter(new(log.TextFormatter))
	default:
		log.Fatalf("log-format value %s is not a valid option (json or text)", logFormat)
	}

	return &KAFKA{
		kafka:           kafka.NewFromConfig(cfg),
		backend:         cc.NewBackend(),
		instanceFilters: config.ProcessFilters(c.StringSlice("kafka_instance-filter")),
		tagFilters:      config.ProcessFilters(c.StringSlice("kafka_tag-filter")),
		tagCache:        cache.New(c.Duration("kafka-tag-cache-time"), 10*time.Minute),
		checkInterval:   c.GlobalDuration("check-interval"),
		quitCh:          make(chan int),
		onDuplicate:     c.GlobalString("on-duplicate"),
		servicePrefix:   c.String("kafka_consul-service-prefix"),
		serviceSuffix:   c.String("kafka_consul-service-suffix"),
		consulNodeName:  c.String("kafka_consul-node-name"),
	}
}

// Run ...
func (r *KAFKA) Run() {
	log.Debug("Starting KAFKA app")

	allInstances := observer.NewProperty(nil)
	filteredInstances := observer.NewProperty(nil)
	catalogState := &config.CatalogState{}

	go r.backend.CatalogReader(catalogState, r.consulNodeName, r.quitCh)
	go r.reader(allInstances)
	go r.filter(allInstances, filteredInstances)
	go r.writer(filteredInstances, catalogState)

	<-r.quitCh
}
