function:
	echo $(name)
	GOOS=linux GOARCH=amd64 go build -o services/$(name)/$(name) services/$(name)/$(name).go
	zip services/$(name)/$(name).zip services/$(name)/$(name)
	aws s3 cp services/$(name)/$(name).zip s3://pelodata-serverless
	aws lambda update-function-code --function-name pelodata-$(name) --s3-bucket pelodata-serverless --s3-key $(name).zip

.PHONY: function
