package shared

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

var validPathParams = []string{"challengeId", "programId", "recommendationId"}

func DeleteByID(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
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

	fmt.Printf("table region: %s\n", tableRegion)
	fmt.Printf("table name: %s\n", tableName)

	dataType := ""
	id := ""
	for _, p := range validPathParams {
		if val, ok := request.PathParameters[p]; ok {
			dataType = strings.Replace(p, "Id", "", 1)
			id = strings.TrimSpace(val)
			break
		}
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
			"Id": {S: aws.String(id)},
		},
	}
	getItemOutput, err := db.GetItem(getItemInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to get %s: %s"
		}`, http.StatusInternalServerError, dataType, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       errBody,
		}, nil
	}

	createdBy, ok := getItemOutput.Item["CreatedBy"]
	if !ok || createdBy == nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": The %s doesn't exist
		}`, http.StatusBadRequest, dataType)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	if *createdBy.S != userID {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": Must be the owner of the %s to delete it
		}`, http.StatusUnauthorized, dataType)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       errBody,
		}, nil
	}

	deleteItemInput := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"Id": {S: aws.String(id)},
		},
	}
	_, err = db.DeleteItem(deleteItemInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to delete %s: %s"
		}`, http.StatusInternalServerError, dataType, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       errBody,
		}, nil
	}

	resBody := fmt.Sprintf(`{
		"status": %d,
		"message": "%s deleted"
	}`, http.StatusOK, dataType)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(resBody),
	}, nil
}
