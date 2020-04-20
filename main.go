package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

const (
	hcidumpDataStart = 0x3E
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

func main() {
	var influxAddr string
	var influxDatabase string
	flag.StringVar(&influxAddr, "influx-addr", "", "address to influxdb for storing measurements")
	flag.StringVar(&influxDatabase, "influx-db", "ruuvi", "name of the influx database")
	flag.Parse()

	done := make(chan error, 1)
	data := make(chan []byte, 100)

	hciscan := NewProcess("hcitool lescan --duplicates --passive", done, false, data)
	err := hciscan.Start()
	if err != nil {
		log.Println("unable to start hciscan process:", err)
		return
	}
	hcidump := NewProcess("hcidump --raw", done, true, data)
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
				if influxAddr != "" {
					buf := new(bytes.Buffer)
					_, err := buf.WriteString(m.InfluxLineProtocol())
					if err != nil {
						log.Println("failed to write line to buffer", err)
						continue
					}
					resp, err := http.Post(fmt.Sprintf("%s/write?db=%s", influxAddr, influxDatabase), "text/plain", buf)
					if err != nil {
						log.Println("failed to POST data to influx:", err)
						continue
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusNoContent {
						log.Printf("unexpected return code, expected %d but got %d", http.StatusNoContent, resp.StatusCode)
						reasons, err := ioutil.ReadAll(resp.Body)
						if err != nil {
							log.Println("could not read body")
							continue
						}
						log.Println(string(reasons))
						continue
					}
				}
			}
		case <-done:
			log.Println("thread completed, exiting...")
			return
		}
	}
}
