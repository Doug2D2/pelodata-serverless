package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Doug2D2/pelodata-serverless/services/shared"
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

func getPathParams(url string, request events.APIGatewayV2HTTPRequest) (string, error) {
	userID, ok := request.PathParameters["userId"]
	userID = strings.TrimSpace(userID)
	if !ok || userID == "" {
		return "", errors.New("Path parameter user_id is required: /getUserInfo/{user_id}")
	}

	url = fmt.Sprintf("%s/%s", url, userID)

	return url, nil
}

// getUser returns the user's Peloton user id based on their username or email and password
func getUser(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	method := "GET"
	url := "/api/user"
	var err error

	url, err = getPathParams(url, request)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "%s"
		}`, http.StatusBadRequest, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	body, respHeaders, resCode, err := shared.PelotonRequest(method, url, nil, nil)
	if err != nil {
		res := events.APIGatewayProxyResponse{
			StatusCode: resCode,
			Body:       err.Error(),
		}

		if body != nil {
			res.Body = string(body)
		}

		return res, nil
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
		MultiValueHeaders: respHeaders,
		Body:              string(reply),
	}, nil
}

func main() {
	lambda.Start(getUser)
}
