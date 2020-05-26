package shared

import (
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// GetDBInfo returns the db region and table name from the env vars
func GetDBInfo() (string, string, error) {
	region, exists := os.LookupEnv("table_region")
	if !exists {
		errors.New("table_region env var doesn't exist")
	}
	name, exists := os.LookupEnv("table_name")
	if !exists {
		errors.New("table_name env var doesn't exist")
	}

	return region, name, nil
}

// GetDB returns a DynamoDB instance
func GetDB(region string) *dynamodb.DynamoDB {
	sess := session.Must(session.NewSession())
	config := &aws.Config{
		Endpoint: aws.String(fmt.Sprintf("dynamodb.%s.amazonaws.com", region)),
		Region:   aws.String(region),
	}
	return dynamodb.New(sess, config)
}
