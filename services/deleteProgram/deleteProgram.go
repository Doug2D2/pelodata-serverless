package main

import (
	"context"
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

func deleteProgram(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
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

	programID, ok := request.PathParameters["programId"]
	programID = strings.TrimSpace(programID)
	if !ok || programID == "" {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Path parameter program_id is required: /deleteProgram/{program_id}"
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

	getItemInput := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"Id": {S: aws.String(programID)},
		},
	}
	getItemOutput, err := db.GetItem(getItemInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to get program: %s"
		}`, http.StatusInternalServerError, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       errBody,
		}, nil
	}

	createdBy, ok := getItemOutput.Item["CreatedBy"]
	if !ok || createdBy == nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": The program doesn't exist
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	if *createdBy.S != userID {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": Must be the owner of the program to delete it
		}`, http.StatusUnauthorized)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       errBody,
		}, nil
	}

	deleteItemInput := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"Id": {S: aws.String(programID)},
		},
	}
	_, err = db.DeleteItem(deleteItemInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to delete program: %s"
		}`, http.StatusInternalServerError, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       errBody,
		}, nil
	}

	resBody := fmt.Sprintf(`{
		"status": %d,
		"message": "Program deleted"
	}`, http.StatusOK)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(resBody),
	}, nil
}

func main() {
	lambda.Start(deleteProgram)
}
