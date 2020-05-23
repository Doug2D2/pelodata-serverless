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

type challenge struct {
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
	c := challenge{}
	err := json.Unmarshal([]byte(request.Body), &c)
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

	// Validation on request body
	if c.Name == "" {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "name is required in request body"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	if c.Difficulty <= 0.0 {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "difficulty must be a number greater than 0"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	if c.NumWorkoutGoal < 1 {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "numWorkoutGoal must be a number greater than 0"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	if c.StartDate == "" {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "startDate is required in request body"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	sDate, err := time.Parse("2006-01-02", c.StartDate)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "startDate must be in the format of YYYY-MM-DD"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	if sDate.Before(time.Now()) {
		// StartDate must be after the current date
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "startDate must not be before today"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	if c.EndDate == "" {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "endDate is required in request body"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	eDate, err := time.Parse("2006-01-02", c.EndDate)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "endDate must be in the format of YYYY-MM-DD"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}
	if eDate.Before(sDate) {
		// EndDate must be after the StartDate
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "endDate must not be before startDate"
		}`, http.StatusBadRequest)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil

	}
	if len(c.WorkoutTypes) < 1 {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "workoutTypes must not be empty"
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
			":createdBy": {S: aws.String(userID)},
		}
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

	// If the Scan call returns any items, then that name can't be used
	if len(scanOutput.Items) > 0 {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "A challenge with the name %s already exists"
		}`, http.StatusBadRequest, c.Name)

		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       errBody,
		}, nil
	}

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
	_, err = db.PutItem(putInput)
	if err != nil {
		errBody := fmt.Sprintf(`{
			"status": %d,
			"message": "Unable to save custom challenge: %s"
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
