#!/bin/bash

docker-compose exec influxdb influx -execute "SHOW DATABASES" | grep "ruuvi" && exit

echo "creating database"
docker-compose exec influxdb influx -execute "CREATE DATABASE ruuvi"
docker-compose exec influxdb influx -execute "ALTER RETENTION POLICY \"autogen\" ON \"ruuvi\" DURATION 1h SHARD DURATION 30m"
docker-compose exec influxdb influx -execute "CREATE RETENTION POLICY \"1m\" ON \"ruuvi\" DURATION 24h REPLICATION 1"
docker-compose exec influxdb influx -execute "CREATE RETENTION POLICY \"1h\" ON \"ruuvi\" DURATION INF REPLICATION 1"
docker-compose exec influxdb influx -execute "CREATE CONTINUOUS QUERY \"1m_agg\" ON \"ruuvi\" BEGIN SELECT MEAN(\"temperature\") AS \"temperature\",MEAN(\"battery\") AS \"battery\",MEAN(\"humidity\") AS \"humidity\",MEAN(\"pressure\") AS \"pressure\",MEAN(\"rssi\") AS \"rssi\" INTO \"ruuvi\".\"1m\".\"reading\" FROM \"ruuvi\".\"autogen\".\"reading\" GROUP BY time(1m),sensor END"
docker-compose exec influxdb influx -execute "CREATE CONTINUOUS QUERY \"1h_agg\" ON \"ruuvi\" BEGIN SELECT MEAN(\"temperature\") AS \"temperature\",MEAN(\"battery\") AS \"battery\",MEAN(\"humidity\") AS \"humidity\",MEAN(\"pressure\") AS \"pressure\",MEAN(\"rssi\") AS \"rssi\" INTO \"ruuvi\".\"1h\".\"reading\" FROM \"ruuvi\".\"1m\".\"reading\" GROUP BY time(1h),sensor END"
docker-compose exec influxdb influx -execute "CREATE RETENTION POLICY \"grafana_rp\" ON \"ruuvi\" DURATION INF REPLICATION 1"
docker-compose exec influxdb influx -execute "INSERT INTO \"ruuvi\".\"grafana_rp\" config rp=\"autogen\",gb=\"1m\" 3600000000000"
docker-compose exec influxdb influx -execute "INSERT INTO \"ruuvi\".\"grafana_rp\" config rp=\"1m\",gb=\"1m\" 86400000000000"
docker-compose exec influxdb influx -execute "INSERT INTO \"ruuvi\".\"grafana_rp\" config rp=\"1h\",gb=\"1h\" 9223372036854775806" #-- max ns value in a 64-bit int