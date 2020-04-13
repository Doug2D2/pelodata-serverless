# pelodata-serverless 

A serverless implementation for interacting with the Peloton APIs. Each folder within the `services` folder represents an 
AWS Lambda function. Since Lambda expects code in zip format, once inside the folder you will need to build the go code and 
create a zip file. For example, if you want to update the login function run the following commands from inside the 
`services/login` folder: 

 - GOOS=linux GOARCH=amd64 go build -o login login.go
 - zip login.zip login

You will then need to update the code in Lambda using the AWS console. In the future the goal is to make this an automated 
process on PR merge or commit.
