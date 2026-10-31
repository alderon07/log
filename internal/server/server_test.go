package server

import (
	"context"
	"io"
	"os"
	"net"
	"os"
	"testing"

	api "github.com/alderon07/log/api/v1"
	"github.com/alderon07/log/internal/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestServer(t *testing.T){
	tests := map[string]func(t *testing.T, client api.LogClient, config *Config){
		"produce/consume a message to/from the log succeeds": testProduceConsume,
		"produce/consume stream succeeds": testProduceConsumeStream,
		"consume past log boundary fails": testConsumePastBoundary,	
	}
	
	for scenario, test := range tests {
		t.Run(scenario, func(t *testing.T){
			client, config, teardown := setupTest(t, nil)
			defer teardown()
			test(t, client, config)
		})
	}	
}

func setupTest(t *testing.T, fn func(*Config)) (client api.LogClient, config *Config, teardown func()) {
	// Mark this as a test helper so failures are reported at the caller.
	t.Helper()

	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	clientOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	cc, err := grpc.NewClient(l.Addr().String(), clientOptions...)
	require.NoError(t, err)

	dir, err := os.TempDir()

	return
}
