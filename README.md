# btle_exporter

Bluetooth LE sensor exporter for xiaomi temperature/humidity sensors.

## Hardware

The following are supported

* LYWSDCGQ 
* Xiaomi devices flashed with [ATC](https://github.com/visago/ATC_MiThermometer) firmware

## Names hint file

To aid with labelling the metrics, you can provide a csv file via the `-names-csv` parameter

The file will be in the following format
```
<mac address>,<name>
```

Example
```
A4:C1:38:D0:2C:EC,Unknown
```

## Metrics

The following metrics are available on port 9978 (You can refine it with `--metrics-listen`

```
$ curl -s http://127.0.0.1:9978/metrics |grep -i "btle_"
# HELP btle_exporter_advertisement_count The total number of btle advertisements counted
# TYPE btle_exporter_advertisement_count counter
btle_exporter_advertisement_count 54980
# HELP btle_exporter_advertisement_supported_count The total number of supported btle advertisements counted
# TYPE btle_exporter_advertisement_supported_count counter
btle_exporter_advertisement_supported_count 2751
# HELP btle_exporter_device_advertisement_count Total number of adevertisements detected
# TYPE btle_exporter_device_advertisement_count counter
btle_exporter_device_advertisement_count{mac="a4:c1:38:d0:2c:ec",model="ATC",name="Unknown"} 372
# HELP btle_exporter_device_battery_percent Current battery reading in percent
# TYPE btle_exporter_device_battery_percent gauge
btle_exporter_device_battery_percent{mac="a4:c1:38:d0:2c:ec",model="ATC",name="Unknown"} 66
# HELP btle_exporter_device_count The total number of btle devices detected
# TYPE btle_exporter_device_count counter
btle_exporter_device_count 44
# HELP btle_exporter_device_humidity_percent Current humidity reading in percent
# TYPE btle_exporter_device_humidity_percent gauge
btle_exporter_device_humidity_percent{mac="a4:c1:38:d0:2c:ec",model="ATC",name="Unknown"} 60
# HELP btle_exporter_device_signal_rssi Current signal strength rSSI
# TYPE btle_exporter_device_signal_rssi gauge
btle_exporter_device_signal_rssi{mac="a4:c1:38:d0:2c:ec",model="ATC",name="Unknown"} -46
# HELP btle_exporter_device_supported_count The total number of supported btle devices detected
# TYPE btle_exporter_device_supported_count counter
btle_exporter_device_supported_count 8
# HELP btle_exporter_device_temperature_celcius Current temperature reading in celcius
# TYPE btle_exporter_device_temperature_celcius gauge
btle_exporter_device_temperature_celcius{mac="a4:c1:38:d0:2c:ec",model="ATC",name="Unknown"} 24.4
```

## Installing as a service

There's a sample [./btle_exporter.service](btle_exporter.service) file that
gets installed with `make install`

## Debugging

### Bluetooth stack

```
sudo apt -y install bluez
```

Check if the bluetooth interfaces are available. (And if you have more then one)

```
$ hciconfig
hci1:	Type: Primary  Bus: USB
	BD Address: 00:1A:7D:DA:71:02  ACL MTU: 310:10  SCO MTU: 64:8
	UP RUNNING 
	RX bytes:1148160 acl:0 sco:0 events:29333 errors:0
	TX bytes:2204 acl:0 sco:0 commands:46 errors:0

hci0:	Type: Primary  Bus: USB
	BD Address: 00:00:00:00:00:00  ACL MTU: 0:0  SCO MTU: 0:0
	DOWN 
	RX bytes:43 acl:0 sco:0 events:2 errors:0
	TX bytes:6 acl:0 sco:0 commands:2 errors:0
```
