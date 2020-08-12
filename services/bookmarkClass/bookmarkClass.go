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
//   POST https://api.onepeloton.com/api/favorites/create

type bookmarkRequest struct {
	RideID string `json:"ride_id"`
}

// getBody attempts to generate the request body and return it
// if an error occurs, the error code and message are returned
func getBody(url string, request events.APIGatewayV2HTTPRequest) ([]byte, int, error) {
	bookmarkReq := bookmarkRequest{}

	err := json.Unmarshal([]byte(request.Body), &bookmarkReq)
	bookmarkReq.RideID = strings.TrimSpace(bookmarkReq.RideID)
	if err != nil || bookmarkReq.RideID == "" {
		return nil, http.StatusBadRequest, errors.New("ride_id is required in request body")
	}

	bookmarkBytes, err := json.Marshal(bookmarkReq)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("Unable to marshal request: %s", err)
	}

	return bookmarkBytes, -1, nil
}

// bookmarkClass bookmarks the class that is passed in
func bookmarkClass(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	method := "POST"
	url := "/api/favorites/create"
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

	// Add peloton cookie heade
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
	lambda.Start(bookmarkClass)
}
