package connection_test

import (
	"context"
	"github.com/lhemerly/Constellation/connection"
	"google.golang.org/grpc"
	"sync"
	"testing"
)

func TestGRPCConnection_Concurrency(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.NewGRPCConnection(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to create GRPCConnection: %v", err)
	}

	err = (*conn).Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		(*conn).Send(ctx, []byte("data"))
	}()

	go func() {
		defer wg.Done()
		(*conn).Receive(ctx)
	}()

	go func() {
		defer wg.Done()
		(*conn).Disconnect()
	}()

	wg.Wait()
}
