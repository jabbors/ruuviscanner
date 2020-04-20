#!/bin/bash

docker-compose exec influxdb influx -execute "SHOW DATABASES" | grep "ruuvi" && exit

echo "creating database"
docker-compose exec influxdb influx -execute "CREATE DATABASE ruuvi"
docker-compose exec influxdb influx -execute "ALTER RETENTION POLICY \"autogen\" ON \"ruuvi\" DURATION 1h SHARD DURATION 30m"
docker-compose exec influxdb influx -execute "CREATE RETENTION POLICY \"5m\" ON \"ruuvi\" DURATION 0s REPLICATION 1"
docker-compose exec influxdb influx -execute "CREATE CONTINUOUS QUERY \"cq_basic_br\" ON \"ruuvi\" BEGIN SELECT mean(*) INTO \"ruuvi\".\"5m\".:MEASUREMENT FROM /.*/ GROUP BY time(5m),* END"
