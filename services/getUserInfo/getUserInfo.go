package main

import (
	"context"
	"encoding/json"
	"errors"
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
	if !ok {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
		}, errors.New("user_id must be provided")
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
		}, errors.New("user_id must be provided")
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

	if resp.StatusCode > 399 {
		return events.APIGatewayProxyResponse{
			StatusCode: resp.StatusCode,
		}, fmt.Errorf("Error communicating with Peloton: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to read response body: %s", err)
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
