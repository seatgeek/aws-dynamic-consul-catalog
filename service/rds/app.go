package rds

import (
	"context"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	rds "github.com/aws/aws-sdk-go-v2/service/rds"
	observer "github.com/imkira/go-observer"
	cache "github.com/patrickmn/go-cache"
	cc "github.com/seatgeek/aws-dynamic-consul-catalog/backend/consul"
	config "github.com/seatgeek/aws-dynamic-consul-catalog/config"
	gelf "github.com/seatgeek/logrus-gelf-formatter"
	log "github.com/sirupsen/logrus"
	cli "gopkg.in/urfave/cli.v1"
)

// RDS ...
type RDS struct {
	rds              *rds.Client
	backend          config.Backend
	logger           log.Entry
	instanceFilters  config.Filters
	tagFilters       config.Filters
	tagCache         *cache.Cache
	checkInterval    time.Duration
	quitCh           chan int
	onDuplicate      string
	servicePrefix    string
	serviceSuffix    string
	consulNodeName   string
	consulMasterTag  string
	consulReplicaTag string
}

// New ...
func New(c *cli.Context) *RDS {
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

	return &RDS{
		rds:              rds.NewFromConfig(cfg),
		backend:          cc.NewBackend(),
		instanceFilters:  config.ProcessFilters(c.StringSlice("rds_instance-filter")),
		tagFilters:       config.ProcessFilters(c.StringSlice("rds_tag-filter")),
		tagCache:         cache.New(c.Duration("rds-tag-cache-time"), 10*time.Minute),
		checkInterval:    c.GlobalDuration("check-interval"),
		quitCh:           make(chan int),
		onDuplicate:      c.GlobalString("on-duplicate"),
		servicePrefix:    c.String("rds_consul-service-prefix"),
		serviceSuffix:    c.String("rds_consul-service-suffix"),
		consulNodeName:   c.String("rds_consul-node-name"),
		consulMasterTag:  c.String("rds_consul-master-tag"),
		consulReplicaTag: c.String("rds_consul-replica-tag"),
	}
}

// Run ...
func (r *RDS) Run() {
	log.Info("Starting RDS app")

	allInstances := observer.NewProperty(nil)
	filteredInstances := observer.NewProperty(nil)
	catalogState := &config.CatalogState{}

	go r.backend.CatalogReader(catalogState, r.consulNodeName, r.quitCh)
	go r.reader(allInstances)
	go r.filter(allInstances, filteredInstances)
	go r.writer(filteredInstances, catalogState)

	<-r.quitCh
}
