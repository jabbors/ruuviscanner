# ruuviscanner

ruuviscanner is an application listening for RuuviTag broadcasts and converting them to measurements suitable for storing in InfluxDB.

The code is inspired by https://github.com/Scrin/RuuviCollector, but I needed a more lightweight application that can run on
a [RaspberryPi Zero W](https://www.raspberrypi.org/products/raspberry-pi-zero-w/).

## How to use?

1. Begin with setting up the backend, consisting of influxdb for storing observations and grafana for showing observations.

```
docker-compose up -d
./create-database.sh
```

2. Install dependencies. Note: the scanner only works on Linux.

```
apt install bluez-hcidump
```

3. Build and run the scanner

```
go build && ./ruuviscanner
```

4. Open `http://localhost:3000/ruuvi/` in your browser to view the collected observations.

## Install the scanner on a RaspberryPi Zero W

1. Enable remote access accoding to https://www.raspberrypi.org/documentation/remote-access/ssh/

2. Cross-compile for Linux ARM

```
GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 go build -o ruuviscanner.linux-arm32
```

3. Copy the binary to your pi

```
scp ruuviscanner.linux-arm32 pi@<your-pis-ip>:/usr/local/bin/ruuviscanner
```

4. Install the dependencies on your Pi

```
apt install bluez-hcidump
```

5. Create a systemd unit file to automatically start up at boot

```
cat << EOF > /etc/systemd/system/ruuviscanner.service
[Unit]
Description=RuuviScanner
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root
ExecStart=/usr/local/bin/ruuviscanner -influx-addr http://<your-ip-to-the-influx-backend>:8086
Restart=always

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable ruuviscanner.service
systemctl start ruuviscanner.service
```

# Links

- https://ruuvi.com/setting-up-raspberry-pi-as-a-ruuvi-gateway/
- https://github.com/ruuvi/ruuvi-sensor-protocols
