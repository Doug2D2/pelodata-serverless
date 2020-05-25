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
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
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

func formatOutput(item map[string]*dynamodb.AttributeValue) (customChallenge, error) {
	challenge := customChallenge{}
	var err error

	if item["Id"].S != nil {
		challenge.ID = *item["Id"].S
	}
	if item["CreatedBy"].S != nil {
		challenge.CreatedBy = *item["CreatedBy"].S
	}
	if item["Name"].S != nil {
		challenge.Name = *item["Name"].S
	}
	if item["Description"].S != nil {
		challenge.Description = *item["Description"].S
	}
	if item["Public"].BOOL != nil {
		challenge.Public = *item["Public"].BOOL
	}
	if item["EquipmentNeeded"].SS != nil {
		for _, en := range item["EquipmentNeeded"].SS {
			challenge.EquipmentNeeded = append(challenge.EquipmentNeeded, *en)
		}
	}
	if item["Difficulty"].N != nil {
		diff, err := strconv.ParseFloat(*item["Difficulty"].N, 32)
		if err != nil {
			return customChallenge{}, fmt.Errorf("Unable to convert Difficulty to float: %s", err)
		}
		challenge.Difficulty = float32(diff)
	}
	if item["StartDate"].S != nil {
		challenge.StartDate = *item["StartDate"].S
	}
	if item["EndDate"].S != nil {
		challenge.EndDate = *item["EndDate"].S
	}
	if item["NumWorkoutGoal"].N != nil {
		challenge.NumWorkoutGoal, err = strconv.Atoi(*item["NumWorkoutGoal"].N)
		if err != nil {
			return customChallenge{}, fmt.Errorf("Unable to convert NumWorkoutGoal to int: %s", err)
		}
	}
	if item["WorkoutTypes"].SS != nil {
		for _, wt := range item["WorkoutTypes"].SS {
			challenge.WorkoutTypes = append(challenge.WorkoutTypes, *wt)
		}
	}

	return challenge, nil
}

func getChallengeByID(db *dynamodb.DynamoDB, tableName, userID, challengeID string) (events.APIGatewayProxyResponse, error) {
	getItemInput := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"Id": {S: aws.String(challengeID)},
		},
	}
	getItemOutput, err := db.GetItem(getItemInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to get challenge: %s"
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
			"message": "Unable to find challenge %s"
		}`, http.StatusBadRequest, challengeID)

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
			"message": "Unauthorized to view this challenge"
		}`, http.StatusUnauthorized)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
			Body:       errBody,
		}, nil
	}

	// Format getItemOutput to customProgram
	challenge, err := formatOutput(getItemOutput.Item)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
		}, err
	}

	reply, err := json.Marshal(challenge)
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

func getAllChallenges(db *dynamodb.DynamoDB, tableName, userID string) (events.APIGatewayProxyResponse, error) {
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
			"message": "Unable to get existing challenges: %s"
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
	challenges := []customChallenge{}
	for _, i := range scanOutput.Items {
		c, err := formatOutput(i)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusInternalServerError,
			}, err
		}
		challenges = append(challenges, c)
	}

	reply, err := json.Marshal(challenges)
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

func getChallenges(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
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

	challengeID, _ := request.PathParameters["challengeId"]
	challengeID = strings.TrimSpace(challengeID)

	sess := session.Must(session.NewSession())
	config := &aws.Config{
		Endpoint: aws.String(fmt.Sprintf("dynamodb.%s.amazonaws.com", tableRegion)),
		Region:   aws.String(tableRegion),
	}
	db := dynamodb.New(sess, config)

	if len(challengeID) > 0 {
		return getChallengeByID(db, tableName, userID, challengeID)
	}

	return getAllChallenges(db, tableName, userID)
}

func main() {
	lambda.Start(getChallenges)
}
