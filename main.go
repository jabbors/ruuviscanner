package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pkg/errors"
)

const (
	hcidumpDataStart = 0x3E
	version          = "1.0"
)

type process struct {
	name    string
	cmd     *exec.Cmd
	done    chan<- error
	quit    chan error
	forward bool
	data    chan<- []byte
}

func NewProcess(cmd string, done chan<- error, forward bool, data chan<- []byte) *process {
	p := process{}
	cmdParts := strings.Split(cmd, " ")
	p.name = cmdParts[0]
	if len(cmdParts) > 1 {
		p.cmd = exec.Command(cmdParts[0], cmdParts[1:]...)
	} else {
		p.cmd = exec.Command(cmdParts[0])
	}
	p.done = done
	p.quit = make(chan error, 1)
	p.forward = forward
	p.data = data

	return &p
}

func (p *process) Start() error {
	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if p.forward {
		// Spawn a go routine parsing cmd output and sending it to a channel
		go func(rc io.ReadCloser, d chan<- []byte, q <-chan error) {
			var output []byte
			// var buffer []byte
			for {
				select {
				case <-q:
					return
				default:
					b := make([]byte, 160)
					read, err := rc.Read(b)
					if err != nil {
						if strings.Contains(err.Error(), os.ErrClosed.Error()) {
							return
						}
						log.Println("error reading cmd output:", err)
						return
					}

					for _, row := range bytes.Split(b[0:read], []byte{0x0a}) {
						if len(row) == 0 {
							continue
						}
						if row[0] == 0x3e { // start of incoming packet
							d <- output
							output = bytes.Trim(bytes.ReplaceAll(bytes.ReplaceAll(row, []byte{0x20}, []byte{}), []byte{0x0a}, []byte{}), ">")
						} else if row[0] == 0x3c { // start of outgoing packet
							d <- output
							output = []byte{}
						} else if len(output) > 0 {
							output = append(output, bytes.ReplaceAll(bytes.ReplaceAll(row, []byte{0x20}, []byte{}), []byte{0x0a}, []byte{})...)
						}

					}
				}
			}
		}(stdout, p.data, p.quit)
	}

	// Start the cmd
	err = p.cmd.Start()
	if err != nil {
		close(p.quit)
		return err
	}

	// Spawn a process waiting for the command the finnish
	go func(n string) {
		p.done <- p.cmd.Wait()
		log.Printf("process %s stopped", n)
	}(p.name)
	return nil
}

func (p *process) Stop() error {
	p.quit <- nil
	return p.cmd.Process.Kill()
}

func ensureInfluxDBExists(addr, db string) error {
	r, err := http.Get(fmt.Sprintf("%s/query?db=%s&q=SHOW+RETENTION+POLICIES", addr, db))
	if err != nil {
		return fmt.Errorf("influxdb is not available at %s", addr)
	}
	defer r.Body.Close()

	var influxAPIResponse struct {
		Results []struct {
			StatementID int    `json:"statement_id"`
			Error       string `json:"error"`
		} `json:"results"`
	}

	err = json.NewDecoder(r.Body).Decode(&influxAPIResponse)
	if err != nil {
		return err
	}
	if len(influxAPIResponse.Results) != 1 {
		return fmt.Errorf("something is not right, could not parse influx response")
	}
	if influxAPIResponse.Results[0].Error != "" {
		return fmt.Errorf(influxAPIResponse.Results[0].Error)
	}

	return nil
}

func writeToInflux(addr, database string, m *Measurement) error {
	buf := new(bytes.Buffer)
	_, err := buf.WriteString(m.InfluxLineProtocol())
	if err != nil {
		return errors.Wrap(err, "failed to write data to buffer")
	}
	resp, err := http.Post(fmt.Sprintf("%s/write?db=%s", addr, database), "text/plain", buf)
	if err != nil {
		return errors.Wrap(err, "failed to POST data")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected return code, expected %d but got %d", http.StatusNoContent, resp.StatusCode)
	}
	return nil
}

func setupMqttClient(broker string, port int, username, password string) (mqtt.Client, error) {
	var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
		log.Println("mqtt: connected")
	}

	var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
		log.Printf("mqtt: connection lost: %v", err)
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", broker, port))
	opts.SetClientID(fmt.Sprintf("go_mqtt_client_%d", time.Now().Unix()))
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	return client, nil
}

func publishToMqtt(client mqtt.Client, topicLevel string, m *Measurement) error {
	data, err := m.MqttMessage()
	if err != nil {
		return err
	}
	topic := fmt.Sprintf("%s/%s", topicLevel, m.MAC.String())
	token := client.Publish(topic, 0, false, data)
	token.Wait()
	return nil
}

func main() {
	var influxAddr string
	var influxDatabase string
	var influxEnable bool
	var mqttClient mqtt.Client
	var mqttBroker string
	var mqttPort int
	var mqttUsername string
	var mqttPassword string
	var mqttTopicLevel string
	var mqttPublishDurationInt int
	var mqttPublishDuration time.Duration
	var mqttLastPublish map[string]time.Time
	var mqttEnable bool
	var hcitoolBin, hcidumpBin string
	var versFlag bool
	flag.StringVar(&influxAddr, "influx-addr", "http://localhost:8086", "address to influxdb for storing measurements")
	flag.StringVar(&influxDatabase, "influx-db", "ruuvi", "name of the influx database")
	flag.BoolVar(&influxEnable, "influx-enable", false, "enable storage in influx database")
	flag.StringVar(&mqttBroker, "mqtt-broker", "localhost", "IP or hostname to the MQTT broker")
	flag.IntVar(&mqttPort, "mqtt-port", 1883, "port to the MQTT broker")
	flag.StringVar(&mqttUsername, "mqtt-user", "", "username for MQTT broker")
	flag.StringVar(&mqttPassword, "mqtt-pass", "", "password for MQTT broker")
	flag.StringVar(&mqttTopicLevel, "mqtt-topic-level", "ruuvi", "mqtt topic level")
	flag.IntVar(&mqttPublishDurationInt, "mqtt-publish-duration", 60, "duration (in seconds) between published for each sensor")
	flag.BoolVar(&mqttEnable, "mqtt-enable", false, "enable storage in MQTT")
	flag.StringVar(&hcitoolBin, "hcitool-binary", "/usr/bin/hcitool", "path to hcitool binary")
	flag.StringVar(&hcidumpBin, "hcidump-binary", "/usr/bin/hcidump", "path to hdidump binary")
	flag.BoolVar(&versFlag, "version", false, "print version")
	flag.Parse()

	if versFlag {
		fmt.Printf("ruuviscanner: version=%s\n", version)
		os.Exit(0)
	}

	done := make(chan error, 1)
	data := make(chan []byte, 100)

	var err error

	// ensure that influxdb is accessible and database exists
	if influxEnable {
		err := ensureInfluxDBExists(influxAddr, influxDatabase)
		if err != nil {
			panic(err)
		}
	}

	// ensure the MQTT broker is accessible
	if mqttEnable {
		mqttLastPublish = make(map[string]time.Time)
		mqttPublishDuration, err = time.ParseDuration(fmt.Sprintf("%ds", mqttPublishDurationInt))
		if err != nil {
			panic(err)
		}
		mqttClient, err = setupMqttClient(mqttBroker, mqttPort, mqttUsername, mqttPassword)
		if err != nil {
			panic(err)
		}
	}

	if !influxEnable && !mqttEnable {
		log.Println("No storage backend configured. Specify either -influx-enable or -mqtt-enable")
		return
	}

	// ensure that hcitool/hcidump binaries exists
	if _, err := os.Stat(hcitoolBin); errors.Is(err, os.ErrNotExist) {
		panic(fmt.Sprintf("%s does not exist", hcitoolBin))
	}
	if _, err := os.Stat(hcidumpBin); errors.Is(err, os.ErrNotExist) {
		panic(fmt.Sprintf("%s does not exist", hcidumpBin))
	}

	hciscan := NewProcess(fmt.Sprintf("%s lescan --duplicates --passive", hcitoolBin), done, false, data)
	err = hciscan.Start()
	if err != nil {
		log.Println("unable to start hciscan process:", err)
		return
	}
	hcidump := NewProcess(fmt.Sprintf("%s --raw", hcidumpBin), done, true, data)
	err = hcidump.Start()
	if err != nil {
		log.Println("unable to start hcidump process:", err)
		return
	}

	for {
		select {
		case d := <-data:
			if len(d) == 0 {
				continue
			}
			b := make([]byte, hex.DecodedLen(len(d)))
			_, err := hex.Decode(b, d)
			if err != nil {
				log.Printf("failed decoding hex data: %s, '%s'\n", err, string(d))
				continue
			}
			m, err := NewMeasurement(b)
			if err != nil && err == ErrUnknownDataFormat {
				// log.Printf("ruuvi: looks like a measurement from %s but format is unknown: %s", m.MAC.String(), string(d))
			} else if err == nil {
				log.Printf("measurment: mac: %s; temperature: %2f; humidity: %2f; pressure: %d; battery: %2f; rssi: %d", m.MAC.String(), m.Temperature, m.Humidity, m.Pressure, m.BatteryVoltage, m.RSSI)
				if influxEnable {
					err := writeToInflux(influxAddr, influxDatabase, m)
					if err != nil {
						log.Printf("writing data to influx failed: %s", err)
					}
				}
				if mqttEnable {
					lastPublish, ok := mqttLastPublish[m.MAC.String()]
					if !ok {
						lastPublish = time.Unix(0, 0)
					}
					if time.Since(lastPublish) > mqttPublishDuration {
						err := publishToMqtt(mqttClient, mqttTopicLevel, m)
						if err != nil {
							log.Printf("publishing data to MQTT failed: %s", err)
						} else {
							mqttLastPublish[m.MAC.String()] = time.Now()
						}
					}
				}
			}
		case <-done:
			log.Println("thread completed, exiting...")
			return
		}
	}
}
