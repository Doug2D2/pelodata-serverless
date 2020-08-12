package main

import (
	"bytes"
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
//   POST https://api.onepeloton.com/api/favorites/delete

type unbookmarkRequest struct {
	RideID string `json:"ride_id"`
}

// getBody attempts to generate the request body and return it
// if an error occurs, the error code and message are returned
func getBody(url string, request events.APIGatewayV2HTTPRequest) ([]byte, int, error) {
	unbookmarkReq := unbookmarkRequest{}

	err := json.Unmarshal([]byte(request.Body), &unbookmarkReq)
	unbookmarkReq.RideID = strings.TrimSpace(unbookmarkReq.RideID)
	if err != nil || unbookmarkReq.RideID == "" {
		return nil, http.StatusBadRequest, errors.New("ride_id is required in request body")
	}

	unbookmarkBytes, err := json.Marshal(unbookmarkReq)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("Unable to marshal request: %s", err)
	}

	return unbookmarkBytes, -1, nil
}

// unbookmarkClass unbookmarks the class that is passed in
func unbookmarkClass(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	method := "POST"
	url := "/api/favorites/delete"
	headers := map[string]string{}

	reqBody, resCode, err := getBody(url, request)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "%s"
		}`, resCode, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: resCode,
			Body:       errBody,
		}, nil
	}

	// Add peloton cookie header
	if cookie, ok := request.Headers["Cookie"]; ok {
		headers["Cookie"] = cookie
	}

	body, respHeaders, resCode, err := shared.PelotonRequest(method, url, headers, bytes.NewBuffer(reqBody))
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

	return events.APIGatewayProxyResponse{
		StatusCode:        resCode,
		MultiValueHeaders: respHeaders,
	}, nil
}

func main() {
	lambda.Start(unbookmarkClass)
}
