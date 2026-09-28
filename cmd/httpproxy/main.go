// httpproxy is a reverse proxy that forwards every incoming HTTP request —
// method, headers, body, query, WebSocket upgrades and streamed responses — to
// TARGET, and runs as a native Windows service.
//
// Commands:
//
//	serve    run the proxy in the foreground (default)
//	service  Windows service control: install | uninstall | start | stop | restart | run
package main

import (
	"os"

	"github.com/Sabissimo/http-proxy/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
