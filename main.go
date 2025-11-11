package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/seatgeek/aws-dynamic-consul-catalog/service/rds"
	cli "gopkg.in/urfave/cli.v1"
)

func main() {
	app := cli.NewApp()
	app.Name = "aws-dynamic-consul-catalog"
	app.Usage = "Easily maintain AWS RDS information in Consul service catalog"
	app.Version = "0.1"

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				app.Version = setting.Value
			case "vcs.time":
				if compileTime, err := time.Parse(time.RFC3339, setting.Value); err != nil {
					app.Compiled = compileTime
				}
			}
		}
	}

	app.Flags = []cli.Flag{
		cli.StringSliceFlag{
			Name:   "instance-filter",
			Usage:  "filters to match AWS DB instance fields",
			EnvVar: "INSTANCE_FILTER",
		},
		cli.StringSliceFlag{
			Name:   "tag-filter",
			Usage:  "filters to match AWS DB instance tags",
			EnvVar: "TAG_FILTER",
		},
		cli.StringFlag{
			Name:   "consul-service-prefix",
			Usage:  "Consul catalog service prefix",
			EnvVar: "CONSUL_SERVICE_PREFIX",
			Value:  "",
		},
		cli.StringFlag{
			Name:   "consul-service-suffix",
			Usage:  "Consul catalog service suffix",
			EnvVar: "CONSUL_SERVICE_SUFFIX",
			Value:  "",
		},
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
			Value:  60 * time.Second,
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
	app.Commands = []cli.Command{
		{
			Name:  "rds",
			Usage: "Run the script",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:   "consul-master-tag",
					Usage:  "The Consul service tag for master instances",
					Value:  "master",
					EnvVar: "CONSUL_MASTER_TAG",
				},
				cli.StringFlag{
					Name:   "consul-replica-tag",
					Usage:  "The Consul service tag for replica instances",
					Value:  "replica",
					EnvVar: "CONSUL_REPLICA_TAG",
				},
				cli.StringFlag{
					Name:   "consul-node-name",
					Usage:  "Consul catalog node name",
					Value:  "rds",
					EnvVar: "CONSUL_NODE_NAME",
				},
				cli.DurationFlag{
					Name:   "rds-tag-cache-time",
					Usage:  "The time RDS tags should be cached (eg. 30s, 1h, 1h10m, 1d)",
					EnvVar: "RDS_TAG_CACHE_TIME",
					Value:  30 * time.Minute,
				},
			},
			Action: func(c *cli.Context) error {
				app := rds.New(c)
				app.Run()

				return nil
			},
		},
	}

	app.Run(os.Args)
}
