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
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/google/uuid"
)

type class struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type customProgram struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Public          bool      `json:"public"`
	EquipmentNeeded []string  `json:"equipmentNeeded"`
	NumWeeks        int       `json:"numWeeks"`
	Classes         [][]class `json:"classes"`
	CreatedBy       string    `json:"createdBy"`
	CreatedDate     string    `json:"createdDate"`
}

func addProgram(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	// Get userid from cookie
	cookie, ok := request.Headers["Cookie"]
	if !ok {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Must be logged in to create a program"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	fmt.Printf("COOKIE: %s", cookie)

	// Get db region and name from env
	dbRegion, exists := os.LookupEnv("db_region")
	if !exists {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, errors.New("db_region env var doesn't exist")
	}
	dbName, exists := os.LookupEnv("db_name")
	if !exists {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, errors.New("db_name env var doesn't exist")
	}

	// Parse request body
	cp := customProgram{}
	err := json.Unmarshal([]byte(request.Body), &cp)
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

	id := uuid.New().String()
	cp.Name = strings.TrimSpace(cp.Name)
	cp.Description = strings.TrimSpace(cp.Description)
	classesData, err := json.Marshal(cp.Classes)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, fmt.Errorf("Unable to marshal classes: %s", err)
	}

	// Validation on request body
	if cp.Name == "" {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "name is required in request body"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	if cp.NumWeeks < 1 {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "numWeeks must be a number greater than 0"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

	// TODO: If cp.Public is true, validate that cp.Name is unique for that user
	// If its false, validate that cp.Name is unique for all public programs

	sess := session.Must(session.NewSession())
	config := &aws.Config{
		Endpoint: aws.String(fmt.Sprintf("dynamodb.%s.amazonaws.com", dbRegion)),
		Region:   aws.String(dbRegion),
	}
	db := dynamodb.New(sess, config)

	// TODO: Add createdBy -- Peloton user ID

	inputItem := map[string]*dynamodb.AttributeValue{
		"Id":              {S: aws.String(id)},
		"Name":            {S: aws.String(cp.Name)},
		"Description":     {S: aws.String(cp.Description)},
		"Public":          {BOOL: aws.Bool(cp.Public)},
		"EquipmentNeeded": {SS: aws.StringSlice(cp.EquipmentNeeded)},
		"NumWeeks":        {N: aws.String(strconv.Itoa(cp.NumWeeks))},
		"Classes":         {B: classesData},
		// "CreatedBy":   {S: aws.String("")},
		"CreatedDate": {S: aws.String(time.Now().Format(time.RFC3339))},
	}
	input := &dynamodb.PutItemInput{
		TableName: aws.String(dbName),
		Item:      inputItem,
	}
	_, err = db.PutItem(input)
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

	reply, err := json.Marshal(inputItem)
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
