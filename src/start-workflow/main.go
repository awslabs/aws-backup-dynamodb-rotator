package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
)

// These types will change based on whatever your function needs to
// consume and return. In this case we are passed a dynamodb.CreateTableInput
// object and return the dynamodb.CreateTableOutput from our call to
// dynamodb.CreateTable.

type (
	// Input in this example is an sfn.StartExecutionInput object.
	Input = sfn.StartExecutionInput
	// Output in this example is an sfn.StartExecutionOutput object.
	Output = sfn.StartExecutionOutput
)

func handler(input Input) (Output, error) {
	// Create a Step Functions client named 'svc'
	sess := session.Must(session.NewSession())
	svc := sfn.New(sess)

	// Start the Step Function State Machine
	output, err := svc.StartExecution(&input)
	if err != nil {
		return Output{}, err
	}

	return *output, nil
}

func main() {
	lambda.Start(handler)
}
