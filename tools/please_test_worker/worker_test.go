package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pb "test/proto/worker"
)

var address string

const binaryFile = `#!/bin/sh
mv data.txt $RESULTS_FILE
`
const dataFile = `=== RUN   TestRunTest
--- PASS: TestRunTest (0.00s)
PASS
`

func init() {
	s, lis := startGrpcServer(0, 10000000)
	go s.Serve(lis)
	address = lis.Addr().String()
}

func getClient() pb.TestWorkerClient {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("%s", err)
	}
	return pb.NewTestWorkerClient(conn)
}

func TestRunTest(t *testing.T) {
	response, err := getClient().Test(context.Background(), &pb.TestRequest{
		Rule:     &pb.BuildLabel{PackageName: "tools/please_test_worker", Name: "worker_test"},
		Command:  "$TEST",
		Coverage: true,
		Timeout:  50,
		Binary:   &pb.DataFile{Filename: "test.sh", Contents: []byte(binaryFile)},
		Data:     []*pb.DataFile{&pb.DataFile{Filename: "data.txt", Contents: []byte(dataFile)}},
		Path:     []string{"/usr/local/bin", "/usr/bin", "/bin"},
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(response.Results))
	assert.EqualValues(t, dataFile, response.Results[0])
}
