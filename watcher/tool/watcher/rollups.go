package main

import (
	"context"
	"encoding/json"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/monitoring-app/watcher/lib"
	"github.com/gravitational/trace"
)

func runRollupsWatcher() error {
	kubernetesClient, err := lib.NewKubernetesClient()
	if err != nil {
		return trace.Wrap(err)
	}

	influxDBClient, err := lib.NewInfluxDBClient()
	if err != nil {
		return trace.Wrap(err)
	}

	err = lib.WaitForAPI(context.TODO(), influxDBClient)
	if err != nil {
		return trace.Wrap(err)
	}

	err = influxDBClient.Setup()
	if err != nil {
		return trace.Wrap(err)
	}

	ch := make(chan string)
	go kubernetesClient.WatchConfigMaps(context.TODO(), lib.RollupsPrefix, ch)
	receiveAndCreateRollups(context.TODO(), influxDBClient, ch)
	return nil
}

// receiveAndCreateRollups listens on the provided channel that receives new rollups data and creates
// them in InfluxDB using the provided client
func receiveAndCreateRollups(ctx context.Context, client *lib.InfluxDBClient, ch <-chan string) {
	for {
		select {
		case data, ok := <-ch:
			if !ok {
				log.Warningf("rollups channel closed")
				return
			}

			var rollups []lib.Rollup
			err := json.Unmarshal([]byte(data), &rollups)
			if err != nil {
				log.Errorf("failed to unmarshal: %v %v", data, trace.DebugReport(err))
				continue
			}

			for _, rollup := range rollups {
				err := client.CreateRollup(rollup)
				if err != nil {
					log.Errorf("failed to create rollup: %v %v", rollup, trace.DebugReport(err))
				}
			}
		case <-ctx.Done():
			log.Infof("stopping")
			return
		}
	}
}
