package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Endpoint:
//   GET https://api.onepeloton.com/api/user/{userID}

// Path Params:
//  userID - Peloton user id

type getUserInfoResponse struct {
	UserID        string `json:"id"`
	Username      string `json:"username"`
	Location      string `json:"location"`
	TotalWorkouts int    `json:"total_workouts"`
	WorkoutCounts []struct {
		Name    string `json:"name"`
		Count   int    `json:"count"`
		IconURL string `json:"icon_url"`
	} `json:"workout_counts"`
}

const basePelotonURL = "https://api.onepeloton.com"

// getUser returns the user's Peloton user id based on their username or email and password
func getUser(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	userID, ok := request.PathParameters["userId"]
	userID = strings.TrimSpace(userID)
	if !ok || userID == "" {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Path parameter user_id is required: /getUserInfo/{user_id}"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	url := fmt.Sprintf("%s/api/user/%s", basePelotonURL, userID)

	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to generate http request: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to get user's info from Peloton: %s", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to read response body: %s", err)
	}

	if resp.StatusCode > 399 {
		if body != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: resp.StatusCode,
				Body:       string(body),
			}, nil
		}

		return events.APIGatewayProxyResponse{
			StatusCode: resp.StatusCode,
		}, fmt.Errorf("Error communicating with Peloton: %s", resp.Status)
	}

	getUserInfoRes := &getUserInfoResponse{}
	err = json.Unmarshal(body, getUserInfoRes)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to unmarshal response: %s", err)
	}

	reply, err := json.Marshal(getUserInfoRes)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to marshal response: %s", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode:        http.StatusOK,
		MultiValueHeaders: resp.Header,
		Body:              string(reply),
	}, nil
}

func main() {
	lambda.Start(getUser)
}
