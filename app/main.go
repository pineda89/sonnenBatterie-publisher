package main

import (
	"context"
	"encoding/json"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	battery_address  = mustGetEnv("BATTERY_ADDRESS", "")
	battery_auth_key = mustGetEnv("BATTERY_AUTH_KEY", "")
	influx_address   = mustGetEnv("INFLUX_URL", "")
	influx_auth      = mustGetEnv("INFLUX_AUTH", "")
	influx_bucket    = mustGetEnv("INFLUX_BUCKET", "battery")
	frequency_update = mustGetEnv("FREQUENCY_UPDATE", "5s")

	battery_api_status = "/api/v2/status"
)

func mustGetEnv(s string, defaultValue string) string {
	if val := os.Getenv(s); len(val) != 0 {
		return val
	}
	if len(defaultValue) > 0 {
		return defaultValue
	}
	panic("environment must exists: " + s)
}

type RsStatus struct {
	ApparentOutput            int         `json:"Apparent_output"`
	BackupBuffer              string      `json:"BackupBuffer"`
	BatteryCharging           bool        `json:"BatteryCharging"`
	BatteryDischarging        bool        `json:"BatteryDischarging"`
	ConsumptionAvg            int         `json:"Consumption_Avg"`
	ConsumptionW              int         `json:"Consumption_W"`
	Fac                       float64     `json:"Fac"`
	FlowConsumptionBattery    bool        `json:"FlowConsumptionBattery"`
	FlowConsumptionGrid       bool        `json:"FlowConsumptionGrid"`
	FlowConsumptionProduction bool        `json:"FlowConsumptionProduction"`
	FlowGridBattery           bool        `json:"FlowGridBattery"`
	FlowProductionBattery     bool        `json:"FlowProductionBattery"`
	FlowProductionGrid        bool        `json:"FlowProductionGrid"`
	GridFeedInW               int         `json:"GridFeedIn_W"` // W, watt . Grid Feed in negative is consumption and positive is feed in
	IsSystemInstalled         int         `json:"IsSystemInstalled"`
	OperatingMode             string      `json:"OperatingMode"`
	PacTotalW                 int         `json:"Pac_total_W"` // W, watt . AC Power greater than ZERO is discharging Inverter AC Power less than ZERO is charging
	ProductionW               int         `json:"Production_W"`
	Rsoc                      int         `json:"RSOC"`
	RemainingCapacityWh       int         `json:"RemainingCapacity_Wh"`
	Sac1                      int         `json:"Sac1"`
	Sac2                      interface{} `json:"Sac2"`
	Sac3                      interface{} `json:"Sac3"`
	SystemStatus              string      `json:"SystemStatus"`
	Timestamp                 string      `json:"Timestamp"`
	Usoc                      int         `json:"USOC"`
	Uac                       int         `json:"Uac"`
	Ubat                      int         `json:"Ubat"`
	DischargeNotAllowed       bool        `json:"dischargeNotAllowed"`
	GeneratorAutostart        bool        `json:"generator_autostart"`
}

func (obj *RsStatus) data() map[string]interface{} {
	data := map[string]interface{}{
		"ConsumptionAvg": obj.ConsumptionAvg,
		"ConsumptionW":   obj.ConsumptionW,
		"ProductionW":    obj.ProductionW,
		"Rsoc":           obj.Rsoc,
		"Usoc":           obj.Usoc,
	}
	if obj.GridFeedInW > 0 {
		data["GridFeedW"] = obj.GridFeedInW
		data["GridConsumptionW"] = 0
	} else {
		data["GridFeedW"] = 0
		data["GridConsumptionW"] = obj.GridFeedInW * -1
	}
	if obj.PacTotalW > 0 {
		data["BatteryDischargingW"] = obj.PacTotalW
		data["BatteryChargingW"] = 0
	} else {
		data["BatteryDischargingW"] = 0
		data["BatteryChargingW"] = obj.PacTotalW * -1
	}
	return data
}

func main() {
	log.Println("starting sonnenBatterie-publisher")
	freq, err := time.ParseDuration(frequency_update)
	if err != nil {
		panic("error parsing frequency_update")
	}

	client := influxdb2.NewClient(influx_address, influx_auth)

	writeAPI := client.WriteAPIBlocking("", influx_bucket)

	for {
		if obj, err := getBatteryStatus(); err == nil {
			p := influxdb2.NewPoint(influx_bucket,
				map[string]string{"power": "w"},
				obj.data(),
				time.Now())

			if werr := writeAPI.WritePoint(context.Background(), p); werr != nil {
				log.Println("error writing point", werr)
			}
		} else {
			log.Println("error getting battery status", err)
		}

		time.Sleep(freq)
	}

	client.Close()
}

func getBatteryStatus() (*RsStatus, error) {
	rq, _ := http.NewRequest(http.MethodGet, "http://"+battery_address+battery_api_status, nil)
	rq.Header.Set("Auth-Token", battery_auth_key)

	rs, err := http.DefaultClient.Do(rq)
	if err != nil {
		return nil, err
	}

	cnt, err := ioutil.ReadAll(rs.Body)
	if err != nil {
		return nil, err
	}

	obj := &RsStatus{}
	json.Unmarshal(cnt, obj)

	return obj, nil
}
