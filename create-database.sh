#!/bin/bash

docker-compose exec influxdb influx -execute "SHOW DATABASES" | grep "ruuvi" && exit

echo "creating database"
docker-compose exec influxdb influx -execute "CREATE DATABASE ruuvi"
docker-compose exec influxdb influx -execute "ALTER RETENTION POLICY \"autogen\" ON \"ruuvi\" DURATION 1h SHARD DURATION 30m"
docker-compose exec influxdb influx -execute "CREATE RETENTION POLICY \"day\" ON \"ruuvi\" DURATION 24h REPLICATION 1"
docker-compose exec influxdb influx -execute "CREATE RETENTION POLICY \"week\" ON \"ruuvi\" DURATION 7d REPLICATION 1"
docker-compose exec influxdb influx -execute "CREATE RETENTION POLICY \"month\" ON \"ruuvi\" DURATION INF REPLICATION 1"
docker-compose exec influxdb influx -execute "CREATE CONTINUOUS QUERY \"day_agg\" ON \"ruuvi\" BEGIN SELECT MEAN(\"temperature\") AS \"temperature\",MEAN(\"battery\") AS \"battery\",MEAN(\"humidity\") AS \"humidity\",MEAN(\"pressure\") AS \"pressure\",MEAN(\"rssi\") AS \"rssi\" INTO \"ruuvi\".\"day\".\"reading\" FROM \"ruuvi\".\"autogen\".\"reading\" GROUP BY time(1m),sensor END"
docker-compose exec influxdb influx -execute "CREATE CONTINUOUS QUERY \"week_agg\" ON \"ruuvi\" BEGIN SELECT MEAN(\"temperature\") AS \"temperature\",MEAN(\"battery\") AS \"battery\",MEAN(\"humidity\") AS \"humidity\",MEAN(\"pressure\") AS \"pressure\",MEAN(\"rssi\") AS \"rssi\" INTO \"ruuvi\".\"week\".\"reading\" FROM \"ruuvi\".\"day\".\"reading\" GROUP BY time(10m),sensor END"
docker-compose exec influxdb influx -execute "CREATE CONTINUOUS QUERY \"month_agg\" ON \"ruuvi\" BEGIN SELECT MEAN(\"temperature\") AS \"temperature\",MEAN(\"battery\") AS \"battery\",MEAN(\"humidity\") AS \"humidity\",MEAN(\"pressure\") AS \"pressure\",MEAN(\"rssi\") AS \"rssi\" INTO \"ruuvi\".\"month\".\"reading\" FROM \"ruuvi\".\"week\".\"reading\" GROUP BY time(1h),sensor END"
docker-compose exec influxdb influx -execute "CREATE RETENTION POLICY \"grafana_rp\" ON \"ruuvi\" DURATION INF REPLICATION 1"
docker-compose exec influxdb influx -execute "INSERT INTO \"ruuvi\".\"grafana_rp\" config rp=\"autogen\",gb=\"10s\" 3600000000000"
docker-compose exec influxdb influx -execute "INSERT INTO \"ruuvi\".\"grafana_rp\" config rp=\"day\",gb=\"1m\" 86400000000000"
docker-compose exec influxdb influx -execute "INSERT INTO \"ruuvi\".\"grafana_rp\" config rp=\"week\",gb=\"10m\" 604800000000000"
docker-compose exec influxdb influx -execute "INSERT INTO \"ruuvi\".\"grafana_rp\" config rp=\"month\",gb=\"1h\" 9223372036854775806" #-- max ns value in a 64-bit int