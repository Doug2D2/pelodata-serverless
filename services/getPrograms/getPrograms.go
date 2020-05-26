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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type customProgram struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Public          bool               `json:"public"`
	EquipmentNeeded []string           `json:"equipmentNeeded"`
	NumWeeks        int                `json:"numWeeks"`
	Workouts        [][]shared.Workout `json:"workouts"`
	CreatedBy       string             `json:"createdBy"`
	CreatedDate     string             `json:"createdDate"`
}

func formatOutput(item map[string]*dynamodb.AttributeValue) (customProgram, error) {
	program := customProgram{}
	var err error

	if item["Id"].S != nil {
		program.ID = *item["Id"].S
	}
	if item["Name"].S != nil {
		program.Name = *item["Name"].S
	}
	if item["Description"].S != nil {
		program.Description = *item["Description"].S
	}
	if item["Public"].BOOL != nil {
		program.Public = *item["Public"].BOOL
	}
	if item["CreatedBy"].S != nil {
		program.CreatedBy = *item["CreatedBy"].S
	}
	if item["CreatedDate"].S != nil {
		program.CreatedDate = *item["CreatedDate"].S
	}
	if item["EquipmentNeeded"].SS != nil {
		for _, en := range item["EquipmentNeeded"].SS {
			program.EquipmentNeeded = append(program.EquipmentNeeded, *en)
		}
	}
	if item["NumWeeks"].N != nil {
		program.NumWeeks, err = strconv.Atoi(*item["NumWeeks"].N)
		if err != nil {
			return customProgram{}, fmt.Errorf("Unable to convert NumWeeks to int: %s", err)
		}
	}
	err = json.Unmarshal(item["Workouts"].B, &program.Workouts)
	if err != nil {
		return customProgram{}, fmt.Errorf("Unable to unmarshal response: %s", err)
	}

	return program, nil
}

func getProgramByID(db *dynamodb.DynamoDB, tableName, userID, programID string) (events.APIGatewayProxyResponse, error) {
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

	// Check if item is not found
	if len(getItemOutput.Item) == 0 {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to find program %s"
		}`, http.StatusBadRequest, programID)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	// If either value is nil, won't be ale to dereference in following if statement
	if getItemOutput.Item["Public"].BOOL == nil || getItemOutput.Item["CreatedBy"].S == nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, errors.New("Invalid nil pointer on Public or CreatedBy")
	}

	// If program is not public or created by the user then they don't have access
	if *getItemOutput.Item["Public"].BOOL == false && *getItemOutput.Item["CreatedBy"].S != userID {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unauthorized to view this program"
		}`, http.StatusUnauthorized)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       errBody,
		}, nil
	}

	// Format getItemOutput to customProgram
	program, err := formatOutput(getItemOutput.Item)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, err
	}

	reply, err := json.Marshal(program)
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

func getAllPrograms(db *dynamodb.DynamoDB, tableName, userID string) (events.APIGatewayProxyResponse, error) {
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		ExpressionAttributeNames: map[string]*string{
			"#P": aws.String("Public"),
		},
		FilterExpression: aws.String("#P = :public or CreatedBy = :createdBy"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":public":    {BOOL: aws.Bool(true)},
			":createdBy": {S: aws.String(userID)},
		},
	}
	scanOutput, err := db.Scan(scanInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to get existing programs: %s"
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
	programs := []customProgram{}
	for _, i := range scanOutput.Items {
		p, err := formatOutput(i)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, err
		}
		programs = append(programs, p)
	}

	reply, err := json.Marshal(programs)
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

	tableRegion, tableName, err := shared.GetDBInfo()
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, err
	}

	programID, _ := request.PathParameters["programId"]
	programID = strings.TrimSpace(programID)

	db := shared.GetDB(tableRegion)

	if len(programID) > 0 {
		return getProgramByID(db, tableName, userID, programID)
	}

	return getAllPrograms(db, tableName, userID)
}

func main() {
	lambda.Start(getPrograms)
}
