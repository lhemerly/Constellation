package connection_test

import (
	"context"
	"net"
	"testing"

	"github.com/lhemerly/Constellation/connection"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func init() {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	go func() {
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestGRPCConnection(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.NewGRPCConnection(ctx, "bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to create GRPCConnection: %v", err)
	}

	t.Run("Connect", func(t *testing.T) {
		err := (*conn).Connect(ctx)
		if err != nil {
			t.Errorf("Connect failed: %v", err)
		}
	})

	t.Run("IsConnected", func(t *testing.T) {
		if !(*conn).IsConnected() {
			t.Error("IsConnected returned false, expected true")
		}
	})

	t.Run("GetRemoteAddress", func(t *testing.T) {
		addr := (*conn).GetRemoteAddress()
		if addr != "bufnet" {
			t.Errorf("GetRemoteAddress returned %s, expected bufnet", addr)
		}
	})

	t.Run("Send and Receive", func(t *testing.T) {
		testData := []byte("test data")
		err := (*conn).Send(ctx, testData)
		if err != nil {
			t.Errorf("Send failed: %v", err)
		}

		receivedData, err := (*conn).Receive(ctx)
		if err != nil {
			t.Errorf("Receive failed: %v", err)
		}
		if string(receivedData) != string(testData) {
			t.Errorf("Received data %s, expected %s", string(receivedData), string(testData))
		}
	})

	t.Run("Disconnect", func(t *testing.T) {
		err := (*conn).Disconnect()
		if err != nil {
			t.Errorf("Disconnect failed: %v", err)
		}
		if (*conn).IsConnected() {
			t.Error("IsConnected returned true after Disconnect, expected false")
		}
	})
}

func TestConnectionFactory(t *testing.T) {
	factory := connection.NewConnectionFactory()

	tests := []struct {
		name           string
		connectionType string
		address        string
		wantErr        bool
	}{
		{"Valid gRPC Connection", "grpc", "bufnet", false},
		{"Invalid Connection Type", "invalid", "bufnet", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			conn, err := factory.NewConnection(ctx, tt.connectionType, tt.address, grpc.WithContextDialer(bufDialer), grpc.WithInsecure())

			if (err != nil) != tt.wantErr {
				t.Errorf("NewConnection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && conn == nil {
				t.Errorf("NewConnection() returned nil connection for valid type")
			}
		})
	}
}