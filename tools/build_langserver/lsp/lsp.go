// Package lsp implements the Language Server Protocol for Please BUILD files.
package lsp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/sourcegraph/go-lsp"
	"github.com/sourcegraph/jsonrpc2"
	"gopkg.in/op/go-logging.v1"
)

var log = logging.MustGetLogger("lsp")

// A Handler is a handler suitable for use with jsonrpc2.
type Handler struct {
	methods map[string]method
	root    string
}

type method struct {
	Func   reflect.Value
	Params reflect.Type
}

// NewHandler returns a new Handler.
func NewHandler() *Handler {
	h := &Handler{}
	h.methods = map[string]method{
		"initialize":  h.method(h.initialize),
		"initialized": h.method(h.initialized),
	}
	return h
}

// Handle implements the jsonrpc2.Handler interface
func (h *Handler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	if resp, err := h.handle(req.Method, req.Params); err != nil {
		if err := conn.ReplyWithError(ctx, req.ID, err); err != nil {
			log.Error("Failed to send error response: %s", err)
		}
	} else if err := conn.Reply(ctx, req.ID, resp); err != nil {
		log.Error("Failed to send response: %s", err)
	}
}

// handle is the slightly higher-level handler that deals with individual methods.
func (h *Handler) handle(method string, params *json.RawMessage) (i interface{}, err *jsonrpc2.Error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("Panic in handler for %s: %s", method, r)
			err = &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: fmt.Sprintf("%s", r),
			}
		}
	}()
	m, present := h.methods[method]
	if !present {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeMethodNotFound}
	}
	p := reflect.New(m.Params)
	if err := json.Unmarshal(*params, p.Interface()); err != nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInvalidParams}
	}
	ret := m.Func.Call([]reflect.Value{p.Elem()})
	if err, ok := ret[1].Interface().(error); ok && err != nil {
		return nil, &jsonrpc2.Error{Code: jsonrpc2.CodeInternalError, Message: err.Error()}
	}
	return ret[0].Interface(), nil
}

// method converts a function to a method struct
func (h *Handler) method(f interface{}) method {
	return method{
		Func:   reflect.ValueOf(f),
		Params: reflect.TypeOf(f).In(0),
	}
}

func (h *Handler) initialize(params *lsp.InitializeParams) (*lsp.InitializeResult, error) {
	// This is a bit yucky and stateful, but we only need to do it once.
	if err := os.Chdir(fromURI(params.RootURI)); err != nil {
		return nil, err
	}
	return &lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			TextDocumentSync: &lsp.TextDocumentSyncOptionsOrKind{
				Options: &lsp.TextDocumentSyncOptions{
					OpenClose: true,
					Change:    lsp.TDSKIncremental,
				},
			},
			DocumentFormattingProvider: true,
		},
	}, nil
}

func (h *Handler) initialized(params *struct{}) (*struct{}, error) {
	// Not doing anything here. Unsure right now what this is really for.
	return &struct{}{}, nil
}

// fromURI converts a DocumentURI to a path.
func fromURI(uri lsp.DocumentURI) string {
	if !strings.HasPrefix(string(uri), "file://") {
		panic("invalid uri: " + uri)
	}
	return string(uri[7:])
}

// A Logger provides an interface to our logger.
type Logger struct{}

// Printf implements the jsonrpc2.Logger interface.
func (l Logger) Printf(tmpl string, args ...interface{}) {
	log.Info(tmpl, args...)
}
