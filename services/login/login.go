package main

import (
	"bytes"
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
//   POST https://api.onepeloton.com/auth/login

type loginRequest struct {
	Username string `json:"username_or_email"`
	Password string `json:"password"`
}

type loginResponse struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

const basePelotonURL = "https://api.onepeloton.com"

// login returns the user's Peloton user id based on their username or email and password
func login(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	url := fmt.Sprintf("%s/auth/login", basePelotonURL)

	req := loginRequest{}
	err := json.Unmarshal([]byte(request.Body), &req)
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if err != nil || req.Username == "" || req.Password == "" {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "username and password are required in request body"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	loginBytes, err := json.Marshal(req)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to marshal request: %s", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(loginBytes))
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to login with Peloton: %s", err)
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
		MultiValueHeaders: resp.Header,
		Body:              string(reply),
	}, nil
}

func main() {
	lambda.Start(login)
}
