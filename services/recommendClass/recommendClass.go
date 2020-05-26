package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Doug2D2/pelodata-serverless/services/shared"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/google/uuid"
)

type recommendation struct {
	ID             string         `json:"id"`
	CreatedBy      string         `json:"createdBy"`
	RecommendedFor string         `json:"recommendedFor"`
	Workout        shared.Workout `json:"workout"`
}

func bodyValidation(r recommendation) error {
	if r.RecommendedFor == "" {
		// Must be recommended to someone
		return errors.New("recommendedFor is required in request body")
	}
	if r.RecommendedFor == r.CreatedBy {
		// User shouldn't be able to recommend to their self
		return errors.New("Unable to recommend a class to yourself")
	}

	return nil
}

func recommendationValidation(r recommendation, workoutData []byte, tableName string, db *dynamodb.DynamoDB) (int, error) {
	scanInput := &dynamodb.ScanInput{
		TableName:        aws.String(tableName),
		FilterExpression: aws.String("CreatedBy = :createdBy and RecommendedFor = :recommendedFor and Workout = :workout"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":createdBy":      {S: aws.String(r.CreatedBy)},
			":recommendedFor": {S: aws.String(r.RecommendedFor)},
			":workout":        {B: workoutData},
		},
	}
	scanOutput, err := db.Scan(scanInput)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Unable to get existing recommendations: %s", err.Error())
	}

	// If the Scan call returns any items, then that recommendation already exists
	if len(scanOutput.Items) > 0 {
		return http.StatusBadRequest, errors.New("That recommendation already exists")
	}

	return -1, nil
}

func putItem(r recommendation, workoutData []byte, tableName string, db *dynamodb.DynamoDB) error {
	itemToPut := map[string]*dynamodb.AttributeValue{
		"Id":             {S: aws.String(r.ID)},
		"CreatedBy":      {S: aws.String(r.CreatedBy)},
		"RecommendedFor": {S: aws.String(r.RecommendedFor)},
		"Workout":        {B: workoutData},
	}
	putInput := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      itemToPut,
	}
	_, err := db.PutItem(putInput)
	if err != nil {
		return fmt.Errorf("Unable to save recommendation: %s", err.Error())
	}

	return nil
}

func recommendClass(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
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

	tableRegion, tableName, err := shared.GetDBInfo()
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, err
	}

	// Parse request body
	r := recommendation{}
	err = json.Unmarshal([]byte(request.Body), &r)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Invalid request body"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	r.ID = uuid.New().String()
	r.CreatedBy = userID
	workoutData, err := json.Marshal(r.Workout)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to marshal classes: %s", err)
	}

	err = bodyValidation(r)
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

	db := shared.GetDB(tableRegion)

	if returnCode, err := recommendationValidation(r, workoutData, tableName, db); err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "%s"
		}`, returnCode, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: returnCode,
			Body:       errBody,
		}, nil
	}

	err = putItem(r, workoutData, tableName, db)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "%s"
		}`, http.StatusInternalServerError, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       errBody,
		}, nil
	}

	reply, err := json.Marshal(r)
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

func main() {
	lambda.Start(recommendClass)
}
