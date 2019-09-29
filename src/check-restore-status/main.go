package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// These types will change based on whatever your function needs to
// consume and return. In this case we are passed a dynamodb.CreateTableInput
// object and return the dynamodb.CreateTableOutput from our call to
// dynamodb.CreateTable.

type (
	// Input is our previous RestoreTableFromBackupOutput record
	Input = dynamodb.RestoreTableFromBackupOutput

	// Output in this example is a dynamodb.CreateTableOutput object.
	Output = dynamodb.DescribeTableOutput
)

func handler(_ context.Context, input Input) (Output, error) {
	// Create a DynamoDB client named 'svc'
	sess := session.Must(session.NewSession())
	svc := dynamodb.New(sess)

	// Create our DescribeTableInput record
	dynamoDBInput := dynamodb.DescribeTableInput{
		TableName: input.TableDescription.TableName,
	}

	// Check our restoring/restored table status
	output, err := svc.DescribeTable(&dynamoDBInput)
	if err != nil {
		return Output{}, err
	}

	return *output, nil
}

func main() {
	lambda.Start(handler)
}
