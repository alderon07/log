package server

import (
	"context"
	"net"
	"os"
	"testing"

	api "github.com/alderon07/log/api/v1"
	"github.com/alderon07/log/internal/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestServer(t *testing.T) {
	tests := map[string]func(t *testing.T, client api.LogClient, config *Config){
		"produce/consume a message to/from the log succeeds": testProduceConsume,
		"produce/consume stream succeeds":                    testProduceConsumeStream,
		"consume past log boundary fails":                    testConsumePastBoundary,
	}

	for scenario, test := range tests {
		t.Run(scenario, func(t *testing.T) {
			client, config, teardown := setupTest(t, nil)
			defer teardown()
			test(t, client, config)
		})
	}
}

// setupTest starts a log server for a test and returns its client, config, and
// a teardown function. fn optionally modifies the config before the server starts.
func setupTest(t *testing.T, fn func(*Config)) (client api.LogClient, config *Config, teardown func()) {
	// Mark this as a test helper so failures are reported at the caller.
	t.Helper()

	// Let the OS choose an available port for the test server.
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	clientOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	// Connect the test client to the listener using insecure local credentials.
	cc, err := grpc.NewClient(l.Addr().String(), clientOptions...)
	require.NoError(t, err)

	// Store the test log in a temporary directory.
	dir, err := os.MkdirTemp("", "server-test")
	require.NoError(t, err)

	clog, err := log.NewLog(dir, log.Config{})
	require.NoError(t, err)

	cfg := &Config{
		CommitLog: clog,
	}

	if fn != nil {
		fn(cfg)
	}

	server, err := NewGRPCServer(cfg)
	require.NoError(t, err)

	// The go keyword runs Serve in the background; it keeps handling requests
	// until server.Stop is called.
	go func() {
		server.Serve(l)
	}()

	// Return a client stub and cleanup for resources created by this helper.
	client = api.NewLogClient(cc)
	
	teardown = func() {
		// Stop makes Serve return, which ends its goroutine.
		server.Stop()
		cc.Close()
		l.Close()
		clog.Remove()
	}

	return client, cfg, teardown
}

func testProduceConsume(t *testing.T, client api.LogClient, config *Config){
	ctx := context.Background()

	want := &api.Record{
		Value: []byte("hello world!"),
	}

	produce, err := client.Produce(ctx, &api.ProduceRequest{
		Record: want,
	})
	require.NoError(t, err)

	consume, err := client.Consume(ctx, &api.ConsumeRequest{
		Offset: produce.Offset,
	})
	require.NoError(t, err)

	require.Equal(t, want.Value, consume.Record.Value)
	require.Equal(t, want.Offset, consume.Record.Offset)
}

func testProduceConsumeStream(t *testing.T, client api.LogClient, config *Config){
	ctx := context.Background()

	records := []*api.Record{{
		Value: []byte("first message"),
		Offset: 0,
	},{
		Value: []byte("second message"),
		Offset: 1,
	}}

	{
		stream, err := client.ProduceStream(ctx)
		require.NoError(t, err)

		for offset, record := range records {
			err = stream.Send(&api.ProduceRequest{
				Record: record,
			})
			require.NoError(t, err)

			res, err := stream.Recv()
			require.NoError(t, err)

			if res.Offset != uint64(offset){
				t.Fatalf("got offset:  %d, want: %d", res.Offset, offset)
			}
		}
	}

	{
		stream, err := client.ConsumeStream(ctx, &api.ConsumeRequest{Offset: 0})
		require.NoError(t, err)

		for i, record := range records {
			res, err := stream.Recv()
			require.NoError(t, err)
			require.Equal(t, res.Record, &api.Record{
				Value: record.Value,
				Offset: uint64(i),
			})
		}
	}
}

func testConsumePastBoundary(t *testing.T, client api.LogClient, config *Config){
	ctx := context.Background()

	produce, err := client.Produce(ctx, &api.ProduceRequest{
		Record: &api.Record{
			Value: []byte("hello world"),
		},
	})
	require.NoError(t, err)

	consume, err := client.Consume(ctx, &api.ConsumeRequest{
		Offset: produce.Offset + 1,
	})

	if consume != nil {
		t.Fatal("consume not nil")
	}
	require.Error(t, err)

	got := status.Code(err)
	want := status.Code(api.ErrOffsetOutOfRange{}.GRPCStatus().Err())
	if got != want {
		t.Fatalf("got err: %v, want: %v", got, want)
	}
}