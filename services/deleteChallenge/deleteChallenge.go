package main

import (
	"github.com/Doug2D2/pelodata-serverless/services/shared"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(shared.DeleteByID)
}
