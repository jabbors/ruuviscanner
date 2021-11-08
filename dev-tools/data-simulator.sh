#!/bin/bash

# reading,sensor=%s humidity=%f,temperature=%f,pressure=%d,battery=%f,rssi=%d

while true
do
    sensor="00:11:22:33:44:55"
    humidity="4$((RANDOM % 10)).$((RANDOM % 10))" # float
    temperature="20.$((RANDOM % 10))$((RANDOM % 10))" # float
    pressure="10$((RANDOM % 10))" # integer
    battery="3.3$((RANDOM % 10))" # float
    rssi="89.$((RANDOM % 10))" # rssi
    echo "reading,sensor=$sensor humidity=$humidity,temperature=$temperature,pressure=$pressure,battery=$battery,rssi=$rssi"
    curl -X POST http://localhost:8086/write?db=ruuvi -d "reading,sensor=$sensor humidity=$humidity,temperature=$temperature,pressure=$pressure,battery=$battery,rssi=$rssi"
    sleep 10
done