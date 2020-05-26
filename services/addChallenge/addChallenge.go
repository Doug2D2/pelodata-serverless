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
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/google/uuid"
)

type customChallenge struct {
	ID              string   `json:"id"`
	CreatedBy       string   `json:"createdBy"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Public          bool     `json:"public"`
	EquipmentNeeded []string `json:"equipmentNeeded"`
	Difficulty      float32  `json:"difficulty"`
	StartDate       string   `json:"startDate"`
	EndDate         string   `json:"endDate"`
	NumWorkoutGoal  int      `json:"numWorkoutGoal"`
	WorkoutTypes    []string `json:"workoutTypes"`
}

func bodyValidation(c customChallenge) error {
	// Validation on request body
	if c.Name == "" {
		return errors.New("name is required in request body")
	}
	if c.Difficulty <= 0.0 {
		return errors.New("difficulty must be a number greater than 0")
	}
	if c.NumWorkoutGoal < 1 {
		return errors.New("numWorkoutGoal must be a number greater than 0")
	}
	if c.StartDate == "" {
		return errors.New("startDate is required in request body")
	}
	sDate, err := time.Parse("2006-01-02", c.StartDate)
	if err != nil {
		return errors.New("startDate must be in the format of YYYY-MM-DD")
	}
	if sDate.Before(time.Now()) {
		// StartDate must be after the current date
		return errors.New("startDate must not be before today")
	}
	if c.EndDate == "" {
		return errors.New("endDate is required in request body")
	}
	eDate, err := time.Parse("2006-01-02", c.EndDate)
	if err != nil {
		return errors.New("endDate must be in the format of YYYY-MM-DD")
	}
	if eDate.Before(sDate) {
		// EndDate must be after the StartDate
		return errors.New("endDate must not be before startDate")
	}
	if len(c.WorkoutTypes) < 1 {
		return errors.New("workoutTypes must not be empty")
	}

	return nil
}

func nameValidation(c customChallenge, tableName string, db *dynamodb.DynamoDB) (int, error) {
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		ExpressionAttributeNames: map[string]*string{
			"#N": aws.String("Name"),
		},
	}
	if c.Public {
		// If c.Public is true, the name must be unique for all public challenges
		scanInput.ExpressionAttributeNames["#P"] = aws.String("Public")
		scanInput.FilterExpression = aws.String("#N = :name and #P = :public")
		scanInput.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":name":   {S: aws.String(c.Name)},
			":public": {BOOL: aws.Bool(true)},
		}
	} else {
		// else, the name must be unique for the user's challenges
		scanInput.FilterExpression = aws.String("#N = :name and CreatedBy = :createdBy")
		scanInput.ExpressionAttributeValues = map[string]*dynamodb.AttributeValue{
			":name":      {S: aws.String(c.Name)},
			":createdBy": {S: aws.String(c.CreatedBy)},
		}
	}
	scanOutput, err := db.Scan(scanInput)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("Unable to get existing challenges: %s", err.Error())
	}

	// If the Scan call returns any items, then that name can't be used
	if len(scanOutput.Items) > 0 {
		return http.StatusBadRequest, fmt.Errorf("A challenge with the name %s already exists", c.Name)
	}

	return -1, nil
}

func putItem(c customChallenge, tableName string, db *dynamodb.DynamoDB) error {
	itemToPut := map[string]*dynamodb.AttributeValue{
		"Id":              {S: aws.String(c.ID)},
		"CreatedBy":       {S: aws.String(c.CreatedBy)},
		"Name":            {S: aws.String(c.Name)},
		"Description":     {S: aws.String(c.Description)},
		"Public":          {BOOL: aws.Bool(c.Public)},
		"EquipmentNeeded": {SS: aws.StringSlice(c.EquipmentNeeded)},
		"Difficulty":      {N: aws.String(fmt.Sprintf("%.1f", c.Difficulty))},
		"StartDate":       {S: aws.String(c.StartDate)},
		"EndDate":         {S: aws.String(c.EndDate)},
		"NumWorkoutGoal":  {N: aws.String(strconv.Itoa(c.NumWorkoutGoal))},
		"WorkoutTypes":    {SS: aws.StringSlice(c.WorkoutTypes)},
	}
	putInput := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      itemToPut,
	}
	_, err := db.PutItem(putInput)
	if err != nil {
		return fmt.Errorf("Unable to save custom challenge: %s", err.Error())
	}

	return nil
}

func addChallenge(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
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
	c := customChallenge{}
	err = json.Unmarshal([]byte(request.Body), &c)
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

	c.ID = uuid.New().String()
	c.CreatedBy = userID

	err = bodyValidation(c)
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

	if returnCode, err := nameValidation(c, tableName, db); err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "%s"
		}`, returnCode, err.Error())

		return events.APIGatewayProxyResponse{
			StatusCode: returnCode,
			Body:       errBody,
		}, nil
	}

	err = putItem(c, tableName, db)
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

	reply, err := json.Marshal(c)
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
	lambda.Start(addChallenge)
}
