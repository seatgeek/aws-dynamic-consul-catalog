# aws-dynamic-consul-catalog

`aws-dynamic-consul-catalog [global-config] service [service-config]`

## Docker build

See [Docker Hub - seatgeek/aws-dynamic-consul-catalog](https://hub.docker.com/r/seatgeek/aws-dynamic-consul-catalog/)

## Consul & AWS Configuration

- [Consul configuration uses the normal Consul environment variables](https://www.consul.io/docs/commands/index.html#environment-variables)
- [The project uses the official AWS Go SDK](https://github.com/aws/aws-sdk-go#configuring-credentials), meaning all the usual sources of credentials work (`ENV`, `IAM`, `~/aws/`)

## CLI global configuration

- [optional] `--check-interval=1m` / `CHECK_INTERVAL` How often should we check AWS RDS for changes (examples: `30s, 1h, 1h10m, 1d`)
- [optional] `--consul-service-prefix` / `CONSUL_SERVICE_PREFIX` Prefix your Consul service name with this string.
- [optional] `--consul-service-suffix` / `CONSUL_SERVICE_SUFFIX` Suffix your Consul service name with this string.
- [optional] `--instance-filter key=value` / `INSTANCE_FILTER` Service dependent key/value for filtering on instance properties - Can be used multiple times as CLI argument
- [optional] `--tag-filter key=value` / `TAG_FILTER` Service dependent key/value for filtering on instance tags - Can be used multiple times as CLI argument
- [optional] `--log-level=info` / `LOG_LEVEL` Log verbosity (debug, info, warning, error, fatal)
- [optional] `--on-duplicate-service=ignore-skip-last` / `ON_DUPLICATE_SERVICE` What to do if duplicate services are found in RDS (e.g. multiple instances with same DB name or `consul_service_name` tag and same RDS Replication Role. (`quit`, `ignore`, `ignore-skip-last`)

### Service: RDS

Will every `check-interval` check AWS RDS for changes in the topologies and instances and update the Consul service catalog accordingly

#### RDS : Config

- [optional] `--consul-node-name=rds` / `CONSUL_NODE_NAME` Name the Consul catalog node that all checks will belong to
- [optional] `--consul-master-tag=master` / `CONSUL_MASTER_TAG` The Consul Service tag to use for RDS master instances
- [optional] `--consul-replica-tag=replica` / `CONSUL_REPLICA_TAG` The Consul service tag to use for RDS replica instances
- [optional] `--rds-tag-cache-time=30m` / `RDS_TAG_CACHE_TIME` The time RDS tags should be cached (examples: `30s, 1h, 1h10m, 1d`)

#### RDS : Instance Filters

- `--instance-filter AvailabilityZone=us-east-1e`
- `--instance-filter DBInstanceArn=arn:aws:rds:us-east-1:12345678:db:rds-instance-name`
- `--instance-filter DBInstanceClass=db.m4.large`
- `--instance-filter DBInstanceIdentifier=rds-instance-identifier`
- `--instance-filter DBInstanceStatus=available`
- `--instance-filter Engine=mysql`
- `--instance-filter EngineVersion=5.5.53`
- `--instance-filter VpcID=vpc-12345`

#### RDS : Tag Filters

- `--tag-filter environment=production`

#### RDS : IAM Policy

```
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "Stmt1455741011000",
            "Effect": "Allow",
            "Action": [
                "rds:DescribeDBInstances",
                "rds:ListTagsForResource"
            ],
            "Resource": "*"
        }
    ]
}
```
