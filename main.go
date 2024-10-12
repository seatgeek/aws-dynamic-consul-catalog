package main

import (
	"os"
	"time"

	"github.com/seatgeek/aws-dynamic-consul-catalog/service/elasticache"
	"github.com/seatgeek/aws-dynamic-consul-catalog/service/kafka"
	"github.com/seatgeek/aws-dynamic-consul-catalog/service/rds"
	cli "gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "consul-aws-catalog"
	app.Usage = "Easily maintain AWS Services information in Consul service catalog"
	app.Version = "0.2"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "on-duplicate",
			Usage:  "What to do if duplicate services/check are found in RDS (e.g. multiple instances with same DB name or consul_service_name tag - and same RDS Replication Role",
			EnvVar: "ON_DUPLICATE",
			Value:  "ignore-skip-last",
		},
		cli.DurationFlag{
			Name:   "check-interval",
			Usage:  "How often to check for RDS changes (eg. 30s, 1h, 1h10m, 1d)",
			EnvVar: "CHECK_INTERVAL",
			Value:  300 * time.Second,
		},
		cli.StringFlag{
			Name:   "log-level",
			Usage:  "Define log level",
			EnvVar: "LOG_LEVEL",
			Value:  "info",
		},
		cli.StringFlag{
			Name:   "log-format",
			Usage:  "Define log format",
			EnvVar: "LOG_FORMAT",
			Value:  "text",
		},
	}

	rdsCommand := cli.Command{
		Name:  "rds",
		Usage: "Run the script",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "rds_consul-master-tag",
				Usage:  "The Consul service tag for master instances",
				Value:  "master",
				EnvVar: "RDS_CONSUL_MASTER_TAG",
			},
			cli.StringFlag{
				Name:   "rds_consul-replica-tag",
				Usage:  "The Consul service tag for replica instances",
				Value:  "replica",
				EnvVar: "RDS_CONSUL_REPLICA_TAG",
			},
			cli.StringFlag{
				Name:   "rds_consul-node-name",
				Usage:  "Consul catalog node name",
				Value:  "rds",
				EnvVar: "RDS_CONSUL_NODE_NAME",
			},
			cli.DurationFlag{
				Name:   "rds-tag-cache-time",
				Usage:  "The time RDS tags should be cached (eg. 30s, 1h, 1h10m, 1d)",
				EnvVar: "RDS_TAG_CACHE_TIME",
				Value:  30 * time.Minute,
			},
			cli.StringSliceFlag{
				Name:   "rds_instance-filter",
				Usage:  "AWS filters",
				EnvVar: "RDS_INSTANCE_FILTER",
			},
			cli.StringSliceFlag{
				Name:   "rds_tag-filter",
				Usage:  "AWS tag filters",
				EnvVar: "RDS_TAG_FILTER",
			},
			cli.StringFlag{
				Name:   "rds_consul-service-prefix",
				Usage:  "Consul catalog service prefix",
				EnvVar: "RDS_CONSUL_SERVICE_PREFIX",
				Value:  "",
			},
			cli.StringFlag{
				Name:   "rds_consul-service-suffix",
				Usage:  "Consul catalog service suffix",
				EnvVar: "RDS_CONSUL_SERVICE_SUFFIX",
				Value:  "",
			},
		},
		Action: func(c *cli.Context) error {
			app := rds.New(c)
			app.Run()

			return nil
		},
	}

	kafkaCommand := cli.Command{
		Name:  "kafka",
		Usage: "Run the script",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "kafka_consul-node-name",
				Usage:  "Consul catalog node name",
				Value:  "kafka",
				EnvVar: "KAFKA_CONSUL_NODE_NAME",
			},
			cli.DurationFlag{
				Name:   "kafka-tag-cache-time",
				Usage:  "The time KAFKA tags should be cached (eg. 30s, 1h, 1h10m, 1d)",
				EnvVar: "KAFKA_TAG_CACHE_TIME",
				Value:  30 * time.Minute,
			},
			cli.StringSliceFlag{
				Name:   "kafka_instance-filter",
				Usage:  "AWS filters",
				EnvVar: "KAFKA_INSTANCE_FILTER",
			},
			cli.StringSliceFlag{
				Name:   "kafka_tag-filter",
				Usage:  "AWS tag filters",
				EnvVar: "KAFKA_TAG_FILTER",
			},
			cli.StringFlag{
				Name:   "kafka_consul-service-prefix",
				Usage:  "Consul catalog service prefix",
				EnvVar: "KAFKA_CONSUL_SERVICE_PREFIX",
				Value:  "",
			},
			cli.StringFlag{
				Name:   "kafka_consul-service-suffix",
				Usage:  "Consul catalog service suffix",
				EnvVar: "KAFKA_CONSUL_SERVICE_SUFFIX",
				Value:  "",
			},
		},
		Action: func(c *cli.Context) error {
			app := kafka.New(c)
			app.Run()

			return nil
		},
	}

	elasticacheCommand := cli.Command{
		Name:  "elasticache",
		Usage: "Run the script",
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:   "elasticache_consul-node-name",
				Usage:  "Consul catalog node name",
				Value:  "elasticache",
				EnvVar: "ELASTICACHE_CONSUL_NODE_NAME",
			},
			cli.DurationFlag{
				Name:   "elasticache-tag-cache-time",
				Usage:  "The time KAFKA tags should be cached (eg. 30s, 1h, 1h10m, 1d)",
				EnvVar: "ELASTICACHE_TAG_CACHE_TIME",
				Value:  30 * time.Minute,
			},
			cli.StringSliceFlag{
				Name:   "elasticache_instance-filter",
				Usage:  "AWS filters",
				EnvVar: "ELASTICACHE_INSTANCE_FILTER",
			},
			cli.StringSliceFlag{
				Name:   "elasticache_tag-filter",
				Usage:  "AWS tag filters",
				EnvVar: "ELASTICACHE_TAG_FILTER",
			},
			cli.StringFlag{
				Name:   "elasticache_consul-service-prefix",
				Usage:  "Consul catalog service prefix",
				EnvVar: "ELASTICACHE_CONSUL_SERVICE_PREFIX",
				Value:  "",
			},
			cli.StringFlag{
				Name:   "elasticache_consul-service-suffix",
				Usage:  "Consul catalog service suffix",
				EnvVar: "ELASTICACHE_CONSUL_SERVICE_SUFFIX",
				Value:  "",
			},
		},
		Action: func(c *cli.Context) error {
			app := elasticache.New(c)
			app.Run()

			return nil
		},
	}

	allCommand := cli.Command{
		Name:  "all",
		Usage: "Run all scripts",
		Action: func(c *cli.Context) error {
			rdsApp := rds.New(c)
			kafkaApp := kafka.New(c)
			elasticacheApp := elasticache.New(c)

			go rdsApp.Run()
			go kafkaApp.Run()
			go elasticacheApp.Run()

			// Wait indefinitely
			select {}
		}, Flags: append(
			append(
				rdsCommand.Flags,      // rds command flags
				kafkaCommand.Flags..., // kafka command flags
			),
			elasticacheCommand.Flags..., // elasticache command flags
		),
	}

	app.Commands = []cli.Command{rdsCommand, kafkaCommand, elasticacheCommand, allCommand}

	app.Run(os.Args)
}
