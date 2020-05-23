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
	"github.com/google/uuid"
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
	CreatedBy      string  `json:"createdBy"`
	RecommendedFor string  `json:"recommendedFor"`
	Workout        workout `json:"workout"`
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

	// Parse request body
	r := recommendation{}
	err := json.Unmarshal([]byte(request.Body), &r)
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

	// Validation on request body
	if r.RecommendedFor == "" {
		// Must be recommended to someone
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "recommendedFor is required in request body"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	if r.RecommendedFor == r.CreatedBy {
		// User shouldn't be able to recommend to their self
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to recommend a class to yourself"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	sess := session.Must(session.NewSession())
	config := &aws.Config{
		Endpoint: aws.String(fmt.Sprintf("dynamodb.%s.amazonaws.com", tableRegion)),
		Region:   aws.String(tableRegion),
	}
	db := dynamodb.New(sess, config)

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
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to get existing recommendations: %s"
		}`, http.StatusInternalServerError, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       errBody,
		}, nil
	}

	// If the Scan call returns any items, then that recommendation already exists
	if len(scanOutput.Items) > 0 {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "That recommendation already exists"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

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
	_, err = db.PutItem(putInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to save recommendation: %s"
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
