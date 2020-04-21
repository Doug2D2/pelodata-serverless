package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

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
	url := fmt.Sprintf("%s/api/browse_categories?library_type=on_demand", basePelotonURL)

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
		}, fmt.Errorf("Unable to get categories from Peloton: %s", err)
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
		MultiValueHeaders: resp.Header,
		Body:              string(reply),
	}, nil

}

func main() {
	lambda.Start(getCategories)
}
