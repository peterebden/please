// Package http provides a built-in http server which serves diagnostic information.
package http

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"gopkg.in/op/go-logging.v1"

	"grpcutil"
	cpb "tools/elan/proto/cluster"
)

var log = logging.MustGetLogger("http")

// maxNodeClasses is the maximum number of node classes that we support.
const maxNodeClasses = 22

// A Cluster is a minimal interface that provides the information we need about the cluster.
type Cluster interface {
	GetClusterInfo() *cpb.ClusterInfoResponse
}

// ServeForever starts the HTTP server and serves until killed.
func ServeForever(port int, cluster Cluster) {
	log.Notice("Serving diagnostics on http://127.0.0.1:%d", port)
	h := &handler{
		cluster: cluster,
		nodes:   map[string]string{},
	}
	h.tmpl = template.Must(template.New("html").Funcs(template.FuncMap{
		"className":    h.className,
		"svgPath":      svgPath,
		"svgTransform": svgTransform,
	}).Parse(MustAssetString("index.html")))

	mux := http.NewServeMux()
	mux.HandleFunc("/styles.css", h.Static)
	mux.HandleFunc("/ping", h.Ping)
	mux.HandleFunc("/", h.Serve)
	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
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
	nodes   map[string]string
	mutex   sync.Mutex
}

func (h *handler) Ping(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *handler) Serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if err := h.tmpl.Execute(w, h.cluster.GetClusterInfo()); err != nil {
		log.Error("%s", err)
	}
}

func (h *handler) Static(w http.ResponseWriter, r *http.Request) {
	data, err := Asset(r.URL.Path[1:])
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.URL.Path == "/styles.css" {
		w.Header().Set("Content-Type", "text/css")
	}
	io.Copy(w, bytes.NewReader(data))
}

// className returns the SVG class name for a node.
func (h *handler) className(node string) string {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	if cls, present := h.nodes[node]; present {
		return cls
	}
	cls := "node-" + strconv.Itoa(len(h.nodes)%maxNodeClasses)
	h.nodes[node] = cls
	return cls
}

// svgPath returns an svg path string for the given hash coordinates.
func svgPath(start, end uint64) string {
	// N.B. This slice is always vertical, we rotate it using a transform.
	const r = 400
	const w = 100
	const r2 = r - w
	rad := float64(end-start) * (2 * math.Pi / float64(math.MaxUint64))
	s := math.Sin(rad)
	c := math.Cos(rad)
	x1 := r + r*s
	y1 := r - r*c
	x2 := r + r2*s
	y2 := r - r2*c
	return fmt.Sprintf("M%d,%d L%d,%d A%d,%d 1 0,1 %0.5f,%0.5f L%0.5f,%0.5f A%d,%d 1 0,0 %d,%d",
		r, w, r, 0, r, r, x1, y1, x2, y2, r, r, r, w)
}

// svgTransform returns an svg transform for the given hash coordinates.
func svgTransform(start, end uint64) string {
	deg := float64(start) * (360.0 / float64(math.MaxUint64))
	return fmt.Sprintf("rotate(%0.5f, 400, 400)", deg)
}
