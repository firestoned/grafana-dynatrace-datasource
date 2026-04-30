package main

import (
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/firestoned/grafana-dynatrace-datasource/pkg/plugin"
)

func main() {
	// Serve the plugin. The factory in plugin.NewDatasource is called by
	// Grafana whenever a datasource instance is created or its settings
	// change; instance lifecycle is managed by the SDK.
	if err := datasource.Manage(
		"firestoned-dynatrace-datasource",
		plugin.NewDatasource,
		datasource.ManageOpts{},
	); err != nil {
		log.DefaultLogger.Error("plugin server exited", "err", err.Error())
		os.Exit(1)
	}

	_ = backend.Logger // avoid unused-import noise if pruned
}
