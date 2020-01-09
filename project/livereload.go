package project

import (
	"fmt"
	"net/http"

	"github.com/kataras/neffos"
	"github.com/kataras/neffos/gobwas"
)

type LiveReload struct {
	// Disable set to true to disable browser live reload.
	Disable bool `json:"disable" yaml:"Disable" toml:"Disable"`

	// No, the server should have the localhost everywhere, accept just the port.
	// // Addr is the host:port address of the websocket server.
	// // The javascript file which listens on updates and should be included on the application
	// // is served through: {Addr}/livereload.js.
	// // The websocket endpoint is {Addr}/livereload.
	// //
	// // Defaults to :35729.
	// Addr string `json:"addr" yaml:"Addr" toml:"Addr"`
	Port int `json:"port" yaml:"Port" toml:"Port"`
	ws   *neffos.Server
}

func NewLiveReload() *LiveReload {
	return &LiveReload{
		Port: 35729,
	}
}

func (l *LiveReload) ListenAndServe() error {
	if l.Disable {
		return nil
	}

	if l.Port <= 0 {
		return nil
	}

	l.ws = neffos.New(gobwas.DefaultUpgrader, neffos.Events{
		// Register OnNativeMessage on empty namespace.
		// Communicatation with this server can happen only through browser's native websocket API.
		neffos.OnNativeMessage: func(c *neffos.NSConn, msg neffos.Message) error {
			return nil
		}})

	mux := http.NewServeMux()
	mux.Handle("/livereload", l.ws)
	mux.HandleFunc("/livereload.js", l.HandleJS())

	return http.ListenAndServe(fmt.Sprintf(":%d", l.Port), mux)
}

var reloadMessage = neffos.Message{IsNative: true, Body: []byte("full_reload")}

func (l *LiveReload) SendReloadSignal() {
	if l.Disable {
		return
	}

	l.ws.Broadcast(nil, reloadMessage)
}

// HandleJS serves the /livereload.js.
//
// We handle the javascript side here in order to be
// easier to listen on reload events within any application.
//
// Just add this script before the closing body tag: <script src="http://localhost:35729/livereload.js></script>"
// Note that Iris injects a script like that automatically if it runs under iris-cli, so users don't have to inject that manually.
func (l *LiveReload) HandleJS() http.HandlerFunc {
	livereloadJS := []byte(fmt.Sprintf(`(function () {
    const scheme = document.location.protocol == "https:" ? "wss" : "ws";
    const endpoint = scheme + "://" + document.location.hostname + ":%d/livereload";

    w = new WebSocket(endpoint);
    w.onopen = function () {
        console.info("LiveReload: initialization");
    };
    w.onclose = function () {
        console.info("LiveReload: terminated");
    };
    w.onmessage = function (message) {
        // NOTE: full-reload, at least for the moment. Also if backend changed its port then we will get 404 here. 
        window.location.reload(); 
    };
}());`, l.Port))

	return func(w http.ResponseWriter, r *http.Request) {
		w.Write(livereloadJS)
	}
}
