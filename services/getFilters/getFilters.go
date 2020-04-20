package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Endpoint:
//   GET https://api.onepeloton.com/api/ride/filters

// Query Params:
//   include_icon_images - Whether to inlcude image icons. Should be true or false
//   library_type - Type of classes, live or on_demand
//   browse_category - Workout category. Ex) cycling, yoga, etc.

type filterValue struct {
	Value           string `json:"value"`
	DisplayName     string `json:"display_name"`
	ListOrder       int    `json:"list_order"`
	DisplayImageURL string `json:"display_image_url"`
}

type filter struct {
	Name         string        `json:"name"`
	DisplayName  string        `json:"display_name"`
	Type         string        `json:"type"`
	UserSpecific bool          `json:"user_specific"`
	Values       []filterValue `json:"values"`
}

type sortValue struct {
	Sort string `json:"sort"`
	Desc bool   `json:"desc"`
}

type sort struct {
	Value       sortValue `json:"value"`
	DisplayName string    `json:"display_name"`
	Slug        string    `json:"slug"`
}

type getFiltersResponse struct {
	Filters []filter `json:"filters"`
	Sorts   []sort   `json:"sorts"`
}

const basePelotonURL = "https://api.onepeloton.com"

func getFilters(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	url := fmt.Sprintf("%s/api/ride/filters?", basePelotonURL)

	// Check for query parameters
	if includeIconsStr, ok := request.QueryStringParameters["include_icon_images"]; ok {
		includeIcons, err := strconv.ParseBool(includeIconsStr)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusBadRequest,
			}, fmt.Errorf("include_icon_images parameter must be true or false")
		}
		url = fmt.Sprintf("%sinclude_icon_images=%v&", url, includeIcons)
	}
	if libraryType, ok := request.QueryStringParameters["library_type"]; ok {
		url = fmt.Sprintf("%slibrary_type=%s&", url, libraryType)
	}
	if cat, ok := request.QueryStringParameters["browse_category"]; ok {
		url = fmt.Sprintf("%sbrowse_category=%s&", url, cat)
	}

	url = strings.TrimRight(url, "&")

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
		}, fmt.Errorf("Unable to get filters from Peloton: %s", err)
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

	getFiltersRes := &getFiltersResponse{}
	err = json.Unmarshal(body, getFiltersRes)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to unmarshal response: %s", err)
	}

	reply, err := json.Marshal(getFiltersRes)
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
	lambda.Start(getFilters)
}
