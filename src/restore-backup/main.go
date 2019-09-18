package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// These types will change based on whatever your function needs to
// consume and return. In this case we are passed a dynamodb.CreateTableInput
// object and return the dynamodb.CreateTableOutput from our call to
// dynamodb.CreateTable.

type (
	// Input in this example is a dynamodb.CreateTableInput object.
	Input = dynamodb.RestoreTableFromBackupInput
	// Output in this example is a dynamodb.CreateTableOutput object.
	Output = dynamodb.RestoreTableFromBackupOutput
)

func handler(input Input) (Output, error) {
	// Create a DynamoDB client named 'svc'
	sess := session.Must(session.NewSession())
	svc := dynamodb.New(sess)

	// Create the DynamoDB Table
	output, err := svc.RestoreTableFromBackup(&input)
	if err != nil {
		return Output{}, err
	}

	return *output, nil
}

func main() {
	lambda.Start(handler)
}
