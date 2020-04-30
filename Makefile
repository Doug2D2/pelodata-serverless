function:
	echo $(name)
	GOOS=linux GOARCH=amd64 go build -o $(name) services/$(name)/$(name).go
	zip $(name).zip $(name)
	mv $(name) $(name).zip services/$(name)/output
	aws s3 cp services/$(name)/output/$(name).zip s3://${BUCKET_NAME}
	aws lambda update-function-code --function-name pelodata-$(name) --s3-bucket ${BUCKET_NAME} --s3-key $(name).zip

.PHONY: function
