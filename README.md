# pelodata-serverless 

A serverless implementation for interacting with the Peloton APIs. Each folder within the `services` folder represents an 
AWS Lambda function.  To build and deploy an individual function, run `make name=nameOfFunction` from the project root directory.  This will build the go code, create a zip file, upload it to S3, and update the Lambda function code. 
