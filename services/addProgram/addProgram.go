package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Doug2D2/pelodata-serverless/services/shared"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/google/uuid"
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

func bodyValidation(cp customProgram) error {
	if cp.Name == "" {
		return errors.New("name is required in request body")
	}
	if cp.NumWeeks < 1 {
		return errors.New("numWeeks must be a number greater than 0")
	}
	if len(cp.Workouts) < 1 {
		return errors.New("workouts must not be empty")
	}

	return nil
}

func addProgram(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
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
	cp := customProgram{}
	err = json.Unmarshal([]byte(request.Body), &cp)
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

	cp.ID = uuid.New().String()
	cp.Name = strings.TrimSpace(cp.Name)
	cp.Description = strings.TrimSpace(cp.Description)
	workoutsData, err := json.Marshal(cp.Workouts)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to marshal classes: %s", err)
	}

	err = bodyValidation(cp)
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

	sess := session.Must(session.NewSession())
	config := &aws.Config{
		Endpoint: aws.String(fmt.Sprintf("dynamodb.%s.amazonaws.com", tableRegion)),
		Region:   aws.String(tableRegion),
	}
	db := dynamodb.New(sess, config)

	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		ExpressionAttributeNames: map[string]*string{
			"#N": aws.String("Name"),
		},
	}
	if cp.Public {
		// If cp.Public is true, the name must be unique for all public programs
		scanInput.ExpressionAttributeNames["#P"] = aws.String("Public")
		scanInput.FilterExpression = aws.String("#N = :name and #P = :public")
		scanInput.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":name":   {S: aws.String(cp.Name)},
			":public": {BOOL: aws.Bool(true)},
		}
	} else {
		// else, the name must be unique for the user's programs
		scanInput.FilterExpression = aws.String("#N = :name and CreatedBy = :createdBy")
		scanInput.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":name":      {S: aws.String(cp.Name)},
			":createdBy": {S: aws.String(userID)},
		}
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

	// If the Scan call returns any items, then that name can't be used
	if len(scanOutput.Items) > 0 {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "A program with the name %s already exists"
		}`, http.StatusBadRequest, cp.Name)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	itemToPut := map[string]*dynamodb.AttributeValue{
		"Id":              {S: aws.String(cp.ID)},
		"Name":            {S: aws.String(cp.Name)},
		"Description":     {S: aws.String(cp.Description)},
		"Public":          {BOOL: aws.Bool(cp.Public)},
		"EquipmentNeeded": {SS: aws.StringSlice(cp.EquipmentNeeded)},
		"NumWeeks":        {N: aws.String(strconv.Itoa(cp.NumWeeks))},
		"Workouts":        {B: workoutsData},
		"CreatedBy":       {S: aws.String(userID)},
		"CreatedDate":     {S: aws.String(time.Now().Format(time.RFC3339))},
	}
	putInput := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      itemToPut,
	}
	_, err = db.PutItem(putInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to save custom program: %s"
		}`, http.StatusInternalServerError, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       errBody,
		}, nil
	}

	reply, err := json.Marshal(cp)
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
	lambda.Start(addProgram)
}
