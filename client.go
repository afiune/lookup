package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lacework/go-sdk/lwcomponent/cdk"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func PingCDK(log *zap.SugaredLogger) error {
	address := fmt.Sprintf("localhost:%s", os.Getenv("LW_CDK_SERVER_PORT"))

	log.Infow("connecting to gRPC server", "address", address)
	// Set up a connection to the CDK server
	conn, err := grpc.Dial(address,
		// @afiune we do an insecure connection since we are
		// connecting to the server running on the same machine
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.Wrap(err, "could not connect")
	}
	defer conn.Close()

	var (
		cdkClient   = cdk.NewStatusClient(conn)
		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	)
	defer cancel()

	r, err := cdkClient.Ping(ctx, &cdk.PingRequest{ComponentName: os.Getenv("LW_COMPONENT_NAME")})
	if err != nil {
		return errors.Wrap(err, "could not ping")
	}
	log.Debugw("response", "from", "cdk.Status/Ping", "message", r.GetMessage())
	return nil
}
