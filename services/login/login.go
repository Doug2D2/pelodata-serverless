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
//   POST https://api.onepeloton.com/auth/login

type loginRequest struct {
	Username string `json:"username_or_email"`
	Password string `json:"password"`
}

type loginResponse struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

func getBody(url string, request events.APIGatewayV2HTTPRequest) ([]byte, int, error) {
	loginReq := loginRequest{}
	err := json.Unmarshal([]byte(request.Body), &loginReq)
	loginReq.Username = strings.TrimSpace(loginReq.Username)
	loginReq.Password = strings.TrimSpace(loginReq.Password)
	if err != nil || loginReq.Username == "" || loginReq.Password == "" {
		return nil, http.StatusBadRequest, errors.New("username and password are required in request body")
	}

	loginBytes, err := json.Marshal(loginReq)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("Unable to marshal request: %s", err)
	}

	return loginBytes, -1, nil
}

// login returns the user's Peloton user id based on their username or email and password
func login(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	method := "POST"
	url := "/auth/login"

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

	body, respHeaders, resCode, err := shared.PelotonRequest(method, url, nil, bytes.NewBuffer(reqBody))
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

	loginRes := &loginResponse{}
	err = json.Unmarshal(body, loginRes)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to unmarshal response: %s", err)
	}

	reply, err := json.Marshal(loginRes)
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
	lambda.Start(login)
}
