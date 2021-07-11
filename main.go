package main

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"flag"
	"log"

	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/visago/ble"
	"github.com/visago/ble/linux"
)

const applicationName = "btle_exporter"
const undefined = -99.9

var flagAdapterID string
var flagVerbose bool
var flagVersion bool
var flagDebug bool
var flagMetricsListen string
var flagPIDFile string
var flagNamesCSVFile string

var BuildBranch string
var BuildVersion string
var BuildTime string
var BuildRevision string

type SensorData struct {
	ID                 int
	Type               int
	Model              string
	ModelID            int
	TemperatureCelcius float64
	HumidityPercent    float64
	BatteryPercent     float64
}

var (
	metricsAdvertisementCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "btle_exporter_advertisement_count",
		Help: "The total number of btle advertisements counted",
	})
	metricsAdvertisementSupportedCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "btle_exporter_advertisement_supported_count",
		Help: "The total number of supported btle advertisements counted",
	})
	metricsDeviceCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "btle_exporter_device_count",
		Help: "The total number of btle devices detected",
	})
	metricsDeviceSupportedCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "btle_exporter_device_supported_count",
		Help: "The total number of supported btle devices detected",
	})
	metricsDeviceTemperatureGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "btle_exporter_device_temperature_celcius",
		Help: "Current temperature reading in celcius",
	}, []string{"mac", "name", "model"},
	)
	metricsDeviceHumidityGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "btle_exporter_device_humidity_percent",
		Help: "Current humidity reading in percent",
	}, []string{"mac", "name", "model"},
	)
	metricsDeviceBatteryGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "btle_exporter_device_battery_percent",
		Help: "Current battery reading in percent",
	}, []string{"mac", "name", "model"},
	)
	metricsDeviceSignalGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "btle_exporter_device_signal_rssi",
		Help: "Current signal strength rSSI",
	}, []string{"mac", "name", "model"},
	)
	metricsDeviceAdvertisementCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "btle_exporter_device_advertisement_count",
		Help: "Total number of adevertisements detected",
	}, []string{"mac", "name", "model"},
	)
	metricsDeviceAdvertisementLastSeenGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "btle_exporter_device_advertisement_lastseen_seconds",
		Help: "Unixtimestamp of when the last time advertisment was seen",
	}, []string{"mac", "name", "model"},
	)
)

var discoverMap = make(map[string]bool) // Mac -> Discovered?
var timeOutMap = make(map[string]int64) // Mac -> Discovered?
var namesMap = make(map[string]string)  // MAC -> Name

func bluetoothScan() error {
	d, err := linux.NewDeviceWithName(flagAdapterID)
	if err != nil {
		log.Fatalf("can't new device : %s", err)
	}
	ble.SetDefaultDevice(d)
	log.Printf("Scanning... (forever)")
	ctx := ble.WithSigHandler(context.Background(), nil)
	return ble.Scan(ctx, true, advScanHandler, nil)
}

func advScanHandler(a ble.Advertisement) {
	var flag_connectable string
	if a.Connectable() {
		flag_connectable = "Connectable"
	} else {
		flag_connectable = "NotConnectable"
	}
	advReportData := a.Data()
	sensorData, err := parseAdvertisementReportData(a)
	if err != nil {
		if !discoverMap[a.Addr().String()] || flagDebug { // Consider a bad scan discovered !
			log.Printf("Cannot parse advertisement data : %s", err)
			discoverMap[a.Addr().String()] = true
		}
		return
	}
	name := getMacName(a.Addr().String())
	if sensorData.Model != "Unknown" { // We know how to process the data
		label := prometheus.Labels{"mac": a.Addr().String(), "name": name, "model": sensorData.Model}
		if sensorData.TemperatureCelcius != undefined {
			metricsDeviceTemperatureGauge.With(label).Set(sensorData.TemperatureCelcius)
		}
		if sensorData.HumidityPercent != undefined {
			metricsDeviceHumidityGauge.With(label).Set(sensorData.HumidityPercent)
		}
		if sensorData.BatteryPercent != undefined {
			metricsDeviceBatteryGauge.With(label).Set(sensorData.BatteryPercent)
		}
		metricsDeviceAdvertisementCount.With(label).Inc()
		metricsDeviceSignalGauge.With(label).Set(float64(a.RSSI()))
		metricsDeviceAdvertisementLastSeenGauge.With(label).Set(float64(time.Now().Unix()))
		metricsAdvertisementSupportedCount.Inc()
	}
	timeOutMap[a.Addr().String()] = time.Now().Unix()
	metricsAdvertisementCount.Inc()

	if !discoverMap[a.Addr().String()] || flagDebug {
		if sensorData != nil && sensorData.Model != "Unknown" && sensorData.Model != "Error" && sensorData.Model != "Unsupported" {
			log.Printf("[%s] Name: %s RSSI:%3d Temp:%0.01f Humidity:%0.01f Batt:%0.01f ModelID:0x%04x, ID:%0d Type:%0d [%s %s]",
				a.Addr(), name, a.RSSI(),
				sensorData.TemperatureCelcius,
				sensorData.HumidityPercent,
				sensorData.BatteryPercent,
				sensorData.ModelID, sensorData.ID, sensorData.Type, flag_connectable, sensorData.Model)
			metricsDeviceSupportedCount.Inc()
		} else {
			if flagVerbose {
				log.Printf("[%s] Name: %s RSSI:%3d Data: %s [%0d] [%s %s]", a.Addr(), a.LocalName(), a.RSSI(), hex.EncodeToString(advReportData), len(advReportData), flag_connectable, sensorData.Model)
			}
		}
		discoverMap[a.Addr().String()] = true
		metricsDeviceCount.Inc()
	}
}

func parseAdvertisementReportData(a ble.Advertisement) (*SensorData, error) {
	sensorData := &SensorData{}
	sensorData.Model = "Unknown"
	sensorData.TemperatureCelcius = undefined
	sensorData.HumidityPercent = undefined
	sensorData.BatteryPercent = undefined
	advRawData := a.Data()
	packetPointer := 0
	// https://docs.silabs.com/bluetooth/latest/general/adv-and-scanning/bluetooth-adv-data-basics
	for packetPointer < len(advRawData)-1 {
		advDataLength := int(advRawData[packetPointer])
		advDataModel := int(advRawData[packetPointer+1])
		advData := advRawData[packetPointer+2 : packetPointer+advDataLength+2]
		if advDataModel == 0x16 { // Service Data - Bluetooth Core Specification:Vol. 3, Part C, sections 11.1.10 and 18.10 (v4.0
			if advDataLength >= 18 && advData[0] == byte(0x95) && advData[1] == byte(0xFE) { // Xiaomi / YWSDCGQ - https://github.com/tsymbaliuk/Xiaomi-Thermostat-BLE
				sensorData.Model = "Error"
				sensorData.Type = int(advData[13])
				sensorData.ID = int(advData[6])
				// sensorData.Features = (int(advData[3]) << 8) + int(advData[2])
				sensorData.ModelID = (int(advData[5]) << 8) + int(advData[4])
				data_length := int(advData[15])
				if sensorData.ModelID == 0x01aa { // LYWSDCG
					sensorData.Model = "LYWSDCGQ"
				} else if sensorData.ModelID == 0x045b { // LYWSD02
					sensorData.Model = "Unsupported"
				}
				if sensorData.Type == 0x0D {
					if data_length == 4 && advDataLength == 21 {
						sensorData.TemperatureCelcius = float64((int(advData[17])<<8)+int(advData[16])) / 10
						sensorData.HumidityPercent = float64((int(advData[19])<<8)+int(advData[18])) / 10
					} else if data_length == 4 && advDataLength == 25 {
						sensorData.TemperatureCelcius = float64((int(advData[17])<<8)+int(advData[16])) / 10
						sensorData.HumidityPercent = float64((int(advData[19])<<8)+int(advData[18])) / 10
						sensorData.BatteryPercent = float64(advData[23])
					}
				} else if sensorData.Type == 0x0A && data_length == 1 && advDataLength == 18 {
					sensorData.BatteryPercent = float64(advData[16])
				} else if sensorData.Type == 0x06 {
					if data_length == 2 && advDataLength == 19 {
						sensorData.HumidityPercent = float64((int(advData[17])<<8)+int(advData[16])) / 10
					} else if data_length == 2 && advDataLength == 23 {
						sensorData.HumidityPercent = float64((int(advData[17])<<8)+int(advData[16])) / 10
						sensorData.BatteryPercent = float64(advData[21])
					}
				} else if sensorData.Type == 0x04 {
					if data_length == 2 && advDataLength == 19 {
						sensorData.TemperatureCelcius = float64((int(advData[17])<<8)+int(advData[16])) / 10
					} else if data_length == 2 && advDataLength == 23 {
						sensorData.TemperatureCelcius = float64((int(advData[17])<<8)+int(advData[16])) / 10
						sensorData.BatteryPercent = float64(advData[21])
					}
				}
			} else if advDataLength >= 16 && advData[0] == byte(0x1A) && advData[1] == byte(0x18) { // ATC / https://github.com/atc1441/ATC_MiThermometer
				sensorData.ID = int(advData[14])
				sensorData.Model = "ATC"
				sensorData.TemperatureCelcius = float64((int(advData[8])<<8)+int(advData[9])) / 10
				sensorData.HumidityPercent = float64(advData[10])
				sensorData.BatteryPercent = float64(advData[11])
			}
		}
		packetPointer = packetPointer + advDataLength + 1
	}
	return sensorData, nil
}

func main() {
	log.Printf("%s version %s (Rev: %s Branch: %s) built on %s", applicationName, BuildVersion, BuildRevision, BuildBranch, BuildTime)
	parseFlags()
	if len(flagPIDFile) > 0 {
		deferCleanup() // This installs a handler to remove PID file when we quit
		savePIDFile(flagPIDFile)
	}
	if len(flagMetricsListen) > 0 { // Start metrics engine
		httpServerStart()
	}
	if len(flagNamesCSVFile) > 0 { // Load the names hint file
		loadNamesCSVFile(flagNamesCSVFile)
	}
	bluetoothScan()
	log.Printf("quit")
}

func deferCleanup() { // Installs a handler to perform clean up
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGPIPE)
	go func() {
		<-c
		cleanup()
		os.Exit(1)
	}()
}

func cleanup() {
	if len(flagPIDFile) > 0 {
		os.Remove(flagPIDFile)
	}
	log.Printf("%s perform clean up on process end", applicationName)

}

func parseFlags() {
	flag.StringVar(&flagMetricsListen, "metrics-listen", "0.0.0.0:9978", "metrics listener <host>:<port>") // Recommend 0.0.0.0:9978
	flag.StringVar(&flagAdapterID, "adapterID", "hci0", "hci0")                                            // Default to use hci0 (first bt device)
	flag.StringVar(&flagPIDFile, "pidfile", "", "pidfile")
	flag.StringVar(&flagNamesCSVFile, "names-csv", "", "namesfile")
	flag.BoolVar(&flagVerbose, "verbose", false, "verbose flag")
	flag.BoolVar(&flagDebug, "debug", false, "debug flag")
	flag.BoolVar(&flagVersion, "version", false, "get version")
	flag.Parse()
	if flagDebug {
		flagVerbose = true // Its confusing if flagDebug is on, but flagVerbose isn't
	}
	if flagVersion { // Only print version (We always print version), then exit.
		os.Exit(0)
	}
}

func savePIDFile(pidFile string) {
	file, err := os.Create(pidFile)
	if err != nil {
		log.Fatalf("Unable to create pid file : %v", err)
	}
	defer file.Close()
	pid := os.Getpid()
	if _, err = file.WriteString(strconv.Itoa(pid)); err != nil {
		log.Fatalf("Unable to create pid file : %v", err)
	}
	if flagVerbose {
		log.Printf("Wrote PID %0d to %s", pid, flagPIDFile)
	}
	file.Sync() // flush to disk
}

func loadNamesCSVFile(namesFile string) {
	f, err := os.Open(namesFile)
	if err != nil {
		log.Printf("Failed to open %s - %v", namesFile, err)
		return
	}
	defer f.Close() // this needs to be after the err check

	csvLines, err := csv.NewReader(f).ReadAll()
	if err != nil {
		log.Printf("Failed to parse %s - %v", namesFile, err)
		return
	}
	count := 0
	for _, line := range csvLines {
		namesMap[strings.ToLower(line[0])] = line[1] // .Addr always returns lower case
		count++
	}
	log.Printf("Loaded %0d lines from csv file %s", count, namesFile)
}

func getMacName(mac string) string { // Converts a mac adress to a name
	return namesMap[mac]
}

func httpServerStart() {
	var buildInfoMetric = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "blte_exporter_build_info", Help: "Shows the build info/version",
		ConstLabels: prometheus.Labels{"branch": BuildBranch, "revision": BuildRevision, "version": BuildVersion, "buildTime": BuildTime, "goversion": runtime.Version()}})
	prometheus.MustRegister(buildInfoMetric)
	buildInfoMetric.Set(1)
	http.Handle("/metrics", promhttp.Handler()) // Do we really want this ?
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><a href=/metrics>metrics</a></body></html>"))
	})
	go func() {
		if err := http.ListenAndServe(flagMetricsListen, nil); err != nil {
			log.Fatalf("FATAL: Failed to start metrics http engine - %v", err)
		}
	}()
	log.Printf("%s metrics engine listening on %s", applicationName, flagMetricsListen)
}
