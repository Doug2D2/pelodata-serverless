package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
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

type customProgram struct {
	ID              string      `json:"id"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	Public          bool        `json:"public"`
	EquipmentNeeded []string    `json:"equipmentNeeded"`
	NumWeeks        int         `json:"numWeeks"`
	Workouts        [][]workout `json:"workouts"`
	CreatedBy       string      `json:"createdBy"`
	CreatedDate     string      `json:"createdDate"`
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

func getItemByID(db *dynamodb.DynamoDB, tableName, userID, programID string) (events.APIGatewayProxyResponse, error) {
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

	// Check if no results are returned
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

	// If program is not public or created by the user then they don't have access
	if getItemOutput.Item["Public"].BOOL == aws.Bool(false) && getItemOutput.Item["CreatedBy"].S != &userID {
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

func getAllItems(db *dynamodb.DynamoDB, tableName, userID string) (events.APIGatewayProxyResponse, error) {
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
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to find any programs"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
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

	programID, _ := request.PathParameters["programId"]
	programID = strings.TrimSpace(programID)

	sess := session.Must(session.NewSession())
	config := &aws.Config{
		Endpoint: aws.String(fmt.Sprintf("dynamodb.%s.amazonaws.com", tableRegion)),
		Region:   aws.String(tableRegion),
	}
	db := dynamodb.New(sess, config)

	if len(programID) > 0 {
		return getItemByID(db, tableName, userID, programID)
	}

	return getAllItems(db, tableName, userID)
}

func main() {
	lambda.Start(getPrograms)
}
