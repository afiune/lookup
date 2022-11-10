package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/lacework/go-sdk/api"
	"github.com/lacework/go-sdk/lwlogger"
)

func help() {
	fmt.Println("Use lookup command to search for entities in your environment. Try the argument 'user:root'.")
	os.Exit(1)
}

func main() {
	log := lwlogger.New("").Sugar()

	// Ping CDK Server
	if err := PingCDK(log); err != nil {
		fmt.Println("There was a problem connecting to the CDK server")
		fmt.Println(err.Error())
		os.Exit(1)
	}

	lacework, err := api.NewClient(os.Getenv("LW_ACCOUNT"),
		api.WithSubaccount(os.Getenv("LW_SUBACCOUNT")),
		api.WithApiKeys(os.Getenv("LW_API_KEY"), os.Getenv("LW_API_SECRET")),
		api.WithToken(os.Getenv("LW_API_TOKEN")),
		api.WithApiV2(),
	)
	if err != nil {
		fmt.Println("One or more missing configuration.")
		os.Exit(1)
	}

	if len(os.Args) <= 1 {
		help()
	}

	kv := strings.Split(os.Args[1], ":")
	if len(kv) != 2 {
		help()
	}

	// Search always in the past day
	now := time.Now().UTC()
	before := now.AddDate(0, 0, -1) // 1 day from ago

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
			fmt.Println("\nUnable to load entity. Error: %s", err.Error())
			os.Exit(1)
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
			fmt.Println("\nUnable to load entity. Error: %s", err.Error())
			os.Exit(1)
		}

		if len(response.Data) == 0 {
			fmt.Printf("Machine '%s' not found in your environment.\n", kv[1])
			break
		}

		fmt.Println("Machine Information:")
		jsonOut, err := PrettyStruct(response.Data[0])
		if err != nil {
			fmt.Println("\nUnable to output JSON. Error: %s", err.Error())
			os.Exit(1)
		}

		fmt.Println(jsonOut)
	case "image":
		fmt.Printf("'image' lookup not yet implemented.")
	default:
		fmt.Printf("Unsupported entity. Try one of user, machine, image.")
	}
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
