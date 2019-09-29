.PHONY: deps clean build

deps:
	go get -u ./...

clean: 
	rm -rf ./bin/check-restore-status/check-restore-status
	rm -rf ./bin/restore-backup/restore-backup
	rm -rf ./bin/start-workflow/start-workflow
	rm -rf ./bin/update-ssm-parameter/update-ssm-parameter
	
build:
	GOOS=linux GOARCH=amd64 go build -o ./bin/check-restore-status/check-restore-status ./src/check-restore-status
	GOOS=linux GOARCH=amd64 go build -o ./bin/restore-backup/restore-backup ./src/restore-backup
	GOOS=linux GOARCH=amd64 go build -o ./bin/start-workflow/start-workflow ./src/start-workflow
	GOOS=linux GOARCH=amd64 go build -o ./bin/update-ssm-parameter/update-ssm-parameter ./src/update-ssm-parameter

test:
	go test ./...
