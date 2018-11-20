// Package http provides a built-in http server which serves diagnostic information.
package http

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"time"

	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	cpb "tools/elan/proto/cluster"
)

var log = logging.MustGetLogger("http")

// A Cluster is a minimal interface that provides the information we need about the cluster.
type Cluster interface {
	GetClusterInfo() *cpb.ClusterInfoResponse
}

// ServeForever starts the HTTP server and serves until killed.
func ServeForever(port int, cluster Cluster) {
	log.Notice("Serving diagnostics on http://127.0.0.1:%d", port)
	h := &handler{
		cluster: cluster,
		tmpl:    template.Must(template.New("html").Parse(MustAssetString("index.html"))),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/styles.css", h.Static)
	mux.HandleFunc("/ping", h.Ping)
	mux.HandleFunc("/", h.Serve)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	// Add a cleanup hook if the gRPC server gets shut down.
	// This is what will actually terminate the process (because it will cause ListenAndServe
	// to return, and hence we'll exit this function which will terminate main()).
	grpcutil.AddCleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	})
	if err := srv.ListenAndServe(); err != nil {
		log.Warning("%s", err)
	}
}

type handler struct {
	cluster Cluster
	tmpl    *template.Template
}

func (h *handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *handler) Serve(w http.ResponseWriter, r *http.Request) {
	if err := h.tmpl.Execute(w, h.cluster.GetClusterInfo()); err != nil {
		log.Error("%s", err)
	}
}

func (h *handler) Static(w http.ResponseWriter, r *http.Request) {
	log.Debug("received request to %s", r.URL.Path)
	data, err := Asset(r.URL.Path[1:])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	io.Copy(w, bytes.NewReader(data))
}
