.PHONY: deps clean build

deps:
	go get -u ./...

clean: 
	rm -rf ./bin/restore-backup/restore-backup
	rm -rf ./bin/start-workflow/start-workflow
	
build:
	GOOS=linux GOARCH=amd64 go build -o ./bin/restore-backup/restore-backup ./src/restore-backup
	GOOS=linux GOARCH=amd64 go build -o ./bin/start-workflow/start-workflow ./src/start-workflow

test:
	go test ./...
