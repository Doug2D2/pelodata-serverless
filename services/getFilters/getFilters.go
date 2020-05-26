package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Doug2D2/pelodata-serverless/services/shared"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Endpoint:
//   GET https://api.onepeloton.com/api/ride/filters?library_type=on_demand

// Query Params:
//   include_icon_images - Whether to inlcude image icons. Should be true or false
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

func getQueryParams(url string, request events.APIGatewayV2HTTPRequest) (string, error) {
	if includeIconsStr, ok := request.QueryStringParameters["include_icon_images"]; ok {
		includeIcons, err := strconv.ParseBool(includeIconsStr)
		if err != nil {
			return "", errors.New("include_icon_images must be true or false")
		}
		url = fmt.Sprintf("%sinclude_icon_images=%v&", url, includeIcons)
	}
	if cat, ok := request.QueryStringParameters["browse_category"]; ok {
		url = fmt.Sprintf("%sbrowse_category=%s&", url, cat)
	}

	return strings.TrimRight(url, "&"), nil
}

func getFilters(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	method := "GET"
	url := "/api/ride/filters?library_type=on_demand&"
	var err error

	url, err = getQueryParams(url, request)
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
		MultiValueHeaders: respHeaders,
		Body:              string(reply),
	}, nil
}

func main() {
	lambda.Start(getFilters)
}
