package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/lacework/go-sdk/api"
	cdk "github.com/lacework/go-sdk/cli/cdk/go/proto/v1"
	"github.com/lacework/go-sdk/lwlogger"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := lookup(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR %s\n", err)
		os.Exit(1)
	}
}

func help() error {
	fmt.Println("Use lookup command to search for entities in your environment. Try the argument 'user:root'.")
	return nil
}

func lookup() error {
	log := lwlogger.New("").Sugar()

	// Set up a connection to the CDK server
	log.Infow("connecting to gRPC server", "address", os.Getenv("LW_CDK_TARGET"))
	conn, err := grpc.Dial(os.Getenv("LW_CDK_TARGET"),
		// @afiune we do an insecure connection since we are
		// connecting to the server running on the same machine
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.Wrap(err, "could not connect")
	}
	defer conn.Close()

	var (
		cdkClient   = cdk.NewCoreClient(conn)
		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	)
	defer cancel()

	// Ping the CDK Server
	reply, err := cdkClient.Ping(ctx, &cdk.PingRequest{
		ComponentName: os.Getenv("LW_COMPONENT_NAME"),
	})
	if err != nil {
		return errors.Wrap(err, "could not ping")
	}
	log.Debugw("response", "from", "cdk.v1.Core/Ping", "message", reply.GetMessage())

	lacework, err := api.NewClient(os.Getenv("LW_ACCOUNT"),
		api.WithSubaccount(os.Getenv("LW_SUBACCOUNT")),
		api.WithApiKeys(os.Getenv("LW_API_KEY"), os.Getenv("LW_API_SECRET")),
		api.WithToken(os.Getenv("LW_API_TOKEN")),
		api.WithApiV2(),
	)
	if err != nil {
		return errors.Wrap(err, "One or more missing configuration")
	}

	if len(os.Args) <= 1 {
		return help()
	}

	kv := strings.Split(os.Args[1], ":")
	if len(kv) != 2 {
		return help()
	}

	// Search always in the past day
	now := time.Now().UTC()
	before := now.AddDate(0, 0, -1) // 1 day from ago

	defer func() {
		_, err := cdkClient.Honeyvent(context.Background(), &cdk.HoneyventRequest{
			DurationMs: time.Since(now).Milliseconds(),
			Feature:    "lookup_event",
			FeatureData: map[string]string{
				"search": kv[0],
			},
		})
		if err != nil {
			log.Error("unable to send honeyvent", "error", err)
		}
	}()

	switch kv[0] {
	case "user":
		var response api.UsersEntityResponse
		err = lacework.V2.Entities.Search(&response,
			api.SearchFilter{
				Filters: []api.Filter{{
					Field:      "username",
					Expression: "eq",
					Value:      kv[1],
				}},
				TimeFilter: &api.TimeFilter{
					StartTime: &before,
					EndTime:   &now,
				},
			},
		)
		if err != nil {
			return errors.Wrap(err, "Unable to load entity")
		}

		if len(response.Data) == 0 {
			fmt.Printf("User '%s' not found in your environment.\n", kv[1])
			break
		}

		fmt.Println("The user has been seen in the following machines:\n")
		out := []int{}
		for _, user := range response.Data {
			if !contains(out, user.Mid) {
				out = append(out, user.Mid)
			}
		}

		sort.Ints(out)
		fmt.Println(out)
	case "machine":
		var response api.MachineDetailsEntityResponse
		err = lacework.V2.Entities.Search(&response,
			api.SearchFilter{
				Filters: []api.Filter{{
					Field:      "mid",
					Expression: "eq",
					Value:      kv[1],
				}},
				TimeFilter: &api.TimeFilter{
					StartTime: &before,
					EndTime:   &now,
				},
			},
		)
		if err != nil {
			return errors.Wrap(err, "Unable to load entity")
		}

		if len(response.Data) == 0 {
			fmt.Printf("Machine '%s' not found in your environment.\n", kv[1])
			break
		}

		fmt.Println("Machine Information:")
		jsonOut, err := PrettyStruct(response.Data[0])
		if err != nil {
			return errors.Wrap(err, "Unable to output JSON")
		}

		fmt.Println(jsonOut)
	case "image":
		return errors.New("'image' lookup not yet implemented.")
	default:
		return errors.New("Unsupported entity. Try one of user, machine, image.")
	}

	return nil
}

func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func PrettyStruct(data interface{}) (string, error) {
	val, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		return "", err
	}
	return string(val), nil
}
