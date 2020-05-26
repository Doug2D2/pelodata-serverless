package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Doug2D2/pelodata-serverless/services/shared"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Endpoint:
//   GET https://api.onepeloton.com/api/browse_categories?library_type=on_demand

type category struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	ListOrder      int    `json:"list_order"`
	IconURL        string `json:"icon_url"`
	PortalImageURL string `json:"portal_image_url"`
}

type getCategoriesResponse struct {
	BrowseCategories []category `json:"browse_categories"`
}

const basePelotonURL = "https://api.onepeloton.com"

func getCategories(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	method := "GET"
	url := "/api/browse_categories?library_type=on_demand"

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

	getCategoriesRes := &getCategoriesResponse{}
	err = json.Unmarshal(body, getCategoriesRes)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to unmarshal response: %s", err)
	}

	reply, err := json.Marshal(getCategoriesRes)
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
	lambda.Start(getCategories)
}
