package main

import (
	"context"
	"net"
	"os"

	"github.com/sourcegraph/jsonrpc2"
	"gopkg.in/op/go-logging.v1"

	"github.com/thought-machine/please/src/cli"
	"github.com/thought-machine/please/tools/build_langserver/lsp"
)

var log = logging.MustGetLogger("build_langserver")

var opts = struct {
	Usage     string
	Verbosity cli.Verbosity `short:"v" long:"verbosity" default:"notice" description:"Verbosity of output (higher number = more output)"`
	LogFile   cli.Filepath  `long:"log_file" description:"File to echo full logging output to"`

	Mode string `short:"m" long:"mode" default:"stdio" choice:"stdio" choice:"tcp" description:"Mode of the language server communication"`
	Host string `short:"h" long:"host" default:"127.0.0.1" description:"TCP host to communicate with"`
	Port string `short:"p" long:"port" default:"4040" description:"TCP port to communicate with"`
}{
	Usage: `
build_langserver is a binary shipped with Please that you can use as a language server for build files.

It speaks language server protocol from vscode, you can plugin this binary in your IDE to start the language server.
Currently, it supports autocompletion, goto definition for build_defs, and signature help
`,
}

func main() {
	cli.ParseFlagsOrDie("build_langserver", &opts)
	cli.InitLogging(opts.Verbosity)
	if opts.LogFile != "" {
		cli.InitFileLogging(string(opts.LogFile), opts.Verbosity)
	}
	if err := serve(jsonrpc2.AsyncHandler(langserver.NewHandler())); err != nil {
		log.Fatalf("fail to start server: %s", err)
	}
}

func serve(handler jsonrpc2.Handler) error {
	if opts.Mode == "tcp" {
		lis, err := net.Listen("tcp", opts.Host+":"+opts.Port)
		if err != nil {
			return err
		}
		defer lis.Close()

		log.Notice("build_langserver: listening on", opts.Host+":"+opts.Port)
		for {
			conn, err := lis.Accept()
			if err != nil {
				return err
			}
			<-jsonrpc2.NewConn(context.Background(),
				jsonrpc2.NewBufferedStream(conn, jsonrpc2.VSCodeObjectCodec{}),
				handler,
				jsonrpc2.LogMessages(logger{}),
			).DisconnectNotify()
		}
	} else {
		log.Info("build_langserver: reading on stdin, writing on stdout")

		<-jsonrpc2.NewConn(context.Background(),
			jsonrpc2.NewBufferedStream(stdrwc{}, jsonrpc2.VSCodeObjectCodec{}),
			handler,
			jsonrpc2.LogMessages(logger{}),
		).DisconnectNotify()

		log.Info("connection closed")
	}
	return nil
}

type stdrwc struct{}

func (stdrwc) Read(p []byte) (int, error) {
	return os.Stdin.Read(p)
}

func (stdrwc) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

func (stdrwc) Close() error {
	if err := os.Stdin.Close(); err != nil {
		return err
	}
	return os.Stdout.Close()
}

type logger struct{}

func (l logger) Printf(tmpl string, args ...interface{}) {
	log.Debugf(tmpl, args...)
}
