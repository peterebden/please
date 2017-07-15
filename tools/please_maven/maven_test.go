package maven

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"cli"
)

var server *httptest.Server

func TestAllDependenciesGRPC(t *testing.T) {
	f := NewFetch(server.URL, nil, nil)
	expected := []string{
		"io.grpc:grpc-auth:1.1.2:src:BSD 3-Clause",
		"io.grpc:grpc-core:1.1.2:src:BSD 3-Clause",
		"com.google.guava:guava:20.0:src",
		"com.google.errorprone:error_prone_annotations:2.0.11:src",
		"com.google.code.findbugs:jsr305:3.0.0:src:The Apache Software License, Version 2.0",
		"io.grpc:grpc-context:1.1.2:src:BSD 3-Clause",
		"com.google.instrumentation:instrumentation-api:0.3.0:src:Apache License, Version 2.0",
		"com.google.auth:google-auth-library-credentials:0.4.0:src",
		"io.grpc:grpc-netty:1.1.2:src:BSD 3-Clause",
		"io.netty:netty-codec-http2:4.1.8.Final:src",
		"io.netty:netty-codec-http:4.1.8.Final:src",
		"io.netty:netty-codec:4.1.8.Final:src",
		"io.netty:netty-transport:4.1.8.Final:src",
		"io.netty:netty-buffer:4.1.8.Final:src",
		"io.netty:netty-common:4.1.8.Final:src",
		"io.netty:netty-resolver:4.1.8.Final:src",
		"io.netty:netty-handler:4.1.8.Final:src",
		"com.google.code.gson:gson:2.8.1:src",
		"io.netty:netty-handler-proxy:4.1.8.Final:src",
		"io.netty:netty-codec-socks:4.1.8.Final:src",
		"io.grpc:grpc-okhttp:1.1.2:src:BSD 3-Clause",
		"com.squareup.okhttp:okhttp:2.5.0:src",
		"com.squareup.okio:okio:1.13.0:src",
		"io.grpc:grpc-protobuf:1.1.2:src:BSD 3-Clause",
		"com.google.protobuf:protobuf-java:3.1.0:src",
		"junit:junit:4.12:src:Eclipse Public License 1.0",
		"org.hamcrest:hamcrest-core:1.3:src",
		"org.easymock:easymock:3.4:src",
		"org.objenesis:objenesis:2.2:src",
		"org.easymock:easymockclassextension:3.2:src",
		"com.google.protobuf:protobuf-java-util:3.1.0:src",
		"io.grpc:grpc-protobuf-lite:1.1.2:src:BSD 3-Clause",
		"com.google.protobuf:protobuf-lite:3.0.1:src",
		"io.grpc:grpc-protobuf-nano:1.1.2:src:BSD 3-Clause",
		"com.google.protobuf.nano:protobuf-javanano:3.0.0-alpha-5:src:New BSD license",
		"io.grpc:grpc-stub:1.1.2:src:BSD 3-Clause",
	}
	actual := AllDependencies(f, "io.grpc:grpc-all:1.1.2", false)
	assert.Equal(t, expected, actual)
}

func TestAllDependenciesGRPCWithIndent(t *testing.T) {
	f := NewFetch(server.URL, nil, nil)
	expected := []string{
		"io.grpc:grpc-auth:1.1.2:src:BSD 3-Clause",
		"  io.grpc:grpc-core:1.1.2:src:BSD 3-Clause",
		"    com.google.guava:guava:20.0:src",
		"    com.google.errorprone:error_prone_annotations:2.0.11:src",
		"    com.google.code.findbugs:jsr305:3.0.0:src:The Apache Software License, Version 2.0",
		"    io.grpc:grpc-context:1.1.2:src:BSD 3-Clause",
		"    com.google.instrumentation:instrumentation-api:0.3.0:src:Apache License, Version 2.0",
		"  com.google.auth:google-auth-library-credentials:0.4.0:src",
		"io.grpc:grpc-netty:1.1.2:src:BSD 3-Clause",
		"  io.netty:netty-codec-http2:4.1.8.Final:src",
		"    io.netty:netty-codec-http:4.1.8.Final:src",
		"      io.netty:netty-codec:4.1.8.Final:src",
		"        io.netty:netty-transport:4.1.8.Final:src",
		"          io.netty:netty-buffer:4.1.8.Final:src",
		"            io.netty:netty-common:4.1.8.Final:src",
		"          io.netty:netty-resolver:4.1.8.Final:src",
		"    io.netty:netty-handler:4.1.8.Final:src",
		"    com.google.code.gson:gson:2.8.1:src",
		"  io.netty:netty-handler-proxy:4.1.8.Final:src",
		"    io.netty:netty-codec-socks:4.1.8.Final:src",
		"io.grpc:grpc-okhttp:1.1.2:src:BSD 3-Clause",
		"  com.squareup.okhttp:okhttp:2.5.0:src",
		"    com.squareup.okio:okio:1.13.0:src",
		"io.grpc:grpc-protobuf:1.1.2:src:BSD 3-Clause",
		"  com.google.protobuf:protobuf-java:3.1.0:src",
		"    junit:junit:4.12:src:Eclipse Public License 1.0",
		"      org.hamcrest:hamcrest-core:1.3:src",
		"    org.easymock:easymock:3.4:src",
		"      org.objenesis:objenesis:2.2:src",
		"    org.easymock:easymockclassextension:3.2:src",
		"  com.google.protobuf:protobuf-java-util:3.1.0:src",
		"  io.grpc:grpc-protobuf-lite:1.1.2:src:BSD 3-Clause",
		"    com.google.protobuf:protobuf-lite:3.0.1:src",
		"io.grpc:grpc-protobuf-nano:1.1.2:src:BSD 3-Clause",
		"  com.google.protobuf.nano:protobuf-javanano:3.0.0-alpha-5:src:New BSD license",
		"io.grpc:grpc-stub:1.1.2:src:BSD 3-Clause",
	}
	actual := AllDependencies(f, "io.grpc:grpc-all:1.1.2", true)
	assert.Equal(t, expected, actual)
}

func TestMain(m *testing.M) {
	cli.InitLogging(1) // Suppress informational messages which there can be an awful lot of
	server = httptest.NewServer(http.FileServer(http.Dir("tools/please_maven/test_data")))
	ret := m.Run()
	server.Close()
	os.Exit(ret)
}
