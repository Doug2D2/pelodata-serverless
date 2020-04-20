package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Endpoint:
//   GET https://api.onepeloton.com/api/ride/filters

// Query Params:
//   include_icon_images - Whether to inlcude image icons. Should be true or false
//   library_type - Type of classes, live or on_demand
//   browse_category - Workout category. Ex) cycling, yoga, etc.

const basePelotonURL = "https://api.onepeloton.com"

func getFilters(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	url := fmt.Sprintf("%s/api/ride/filters?", basePelotonURL)

	// Check for query parameters
}

func main() {
	lambda.Start(getFilters)
}
