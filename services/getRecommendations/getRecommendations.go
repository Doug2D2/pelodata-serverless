package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type workout struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Difficulty      float32 `json:"difficulty_estimate"`
	Duration        int     `json:"duration"`
	ImageURL        string  `json:"image_url"`
	InstructorID    string  `json:"instructor_id"`
	InstructorName  string  `json:"instructor_name"`
	OriginalAirTime int64   `json:"original_air_time"`
}

type recommendation struct {
	ID             string  `json:"id"`
	RecommendedBy  string  `json:"recommendedBy"`
	RecommendedFor string  `json:"recommendedFor"`
	Workout        workout `json:"workout"`
}

func formatOutput(item map[string]*dynamodb.AttributeValue) (recommendation, error) {
	rec := recommendation{}
	var err error

	if item["Id"].S != nil {
		rec.ID = *item["Id"].S
	}
	if item["RecommendedBy"].S != nil {
		rec.RecommendedBy = *item["RecommendedBy"].S
	}
	if item["RecommendedFor"].S != nil {
		rec.RecommendedFor = *item["RecommendedFor"].S
	}
	err = json.Unmarshal(item["Workout"].B, &rec.Workout)
	if err != nil {
		return recommendation{}, fmt.Errorf("Unable to unmarshal response: %s", err)
	}

	return rec, nil
}

func getItemByID(db *dynamodb.DynamoDB, tableName, userID, recommendationID string) (events.APIGatewayProxyResponse, error) {
	getItemInput := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"Id": {S: aws.String(recommendationID)},
		},
	}
	getItemOutput, err := db.GetItem(getItemInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to get recommendation: %s"
		}`, http.StatusInternalServerError, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       errBody,
		}, nil
	}

	// Check if item is not found
	if len(getItemOutput.Item) == 0 {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to find recommendation %s"
		}`, http.StatusBadRequest, recommendationID)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	// If either value is nil, won't be ale to dereference in following if statement
	if getItemOutput.Item["RecommendedBy"].S == nil || getItemOutput.Item["RecommendedFor"].S == nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, errors.New("Invalid nil pointer on RecommendedBy or RecommendedFor")
	}

	// recommendedBy or recommendedFor must be the current user
	if *getItemOutput.Item["RecommendedBy"].S != userID && *getItemOutput.Item["RecommendedFor"].S != userID {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unauthorized to view this recommendation"
		}`, http.StatusUnauthorized)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       errBody,
		}, nil
	}

	// Format getItemOutput to customProgram
	recommendation, err := formatOutput(getItemOutput.Item)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, err
	}

	reply, err := json.Marshal(recommendation)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to marshal response: %s", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(reply),
	}, nil
}

func getAllItems(db *dynamodb.DynamoDB, tableName, userID, recType string) (events.APIGatewayProxyResponse, error) {
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	}
	switch recType {
	case "forme":
		scanInput.FilterExpression = aws.String("RecommendedFor = :userID")
		scanInput.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":userID": {S: aws.String(userID)},
		}
	case "byme":
		scanInput.FilterExpression = aws.String("RecommendedBy = :userID")
		scanInput.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":userID": {S: aws.String(userID)},
		}
	case "all":
		scanInput.FilterExpression = aws.String("RecommendedFor = :userID or RecommendedBy = :userID")
		scanInput.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":userID": {S: aws.String(userID)},
		}
	default:
		// Invalid value for type query parameter
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "type must be forMe, byMe, or all"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	scanOutput, err := db.Scan(scanInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to get existing recommendations: %s"
		}`, http.StatusInternalServerError, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       errBody,
		}, nil
	}

	// Check if no results are returned
	if len(scanOutput.Items) == 0 {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusOK,
			Body:       "[]",
		}, nil
	}

	// Format scanOutput to []customProgram
	recs := []recommendation{}
	for _, i := range scanOutput.Items {
		r, err := formatOutput(i)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, err
		}
		recs = append(recs, r)
	}

	reply, err := json.Marshal(recs)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to marshal response: %s", err)
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(reply),
	}, nil
}

func getPrograms(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	// Get UserID header
	userID, ok := request.Headers["UserID"]
	userID = strings.TrimSpace(userID)
	if !ok || userID == "" {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "UserID header is required"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	// Get db region and name from env
	tableRegion, exists := os.LookupEnv("table_region")
	if !exists {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, errors.New("table_region env var doesn't exist")
	}
	tableName, exists := os.LookupEnv("table_name")
	if !exists {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, errors.New("table_name env var doesn't exist")
	}

	// Get path parameter - only used if getting a recommendation by id
	recommendationID, _ := request.PathParameters["recommendationId"]
	recommendationID = strings.TrimSpace(recommendationID)

	// Check for query parameters
	// type - must be either forMe, byMe, or all. Determines the type of recommendations returned
	recType, _ := request.QueryStringParameters["type"]
	recType = strings.ToLower(strings.TrimSpace(recType))
	if recType == "" {
		recType = "forme"
	}

	sess := session.Must(session.NewSession())
	config := &aws.Config{
		Endpoint: aws.String(fmt.Sprintf("dynamodb.%s.amazonaws.com", tableRegion)),
		Region:   aws.String(tableRegion),
	}
	db := dynamodb.New(sess, config)

	if len(recommendationID) > 0 {
		return getItemByID(db, tableName, userID, recommendationID)
	}

	return getAllItems(db, tableName, userID, recType)
}

func main() {
	lambda.Start(getPrograms)
}
