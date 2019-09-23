package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
)

type (
	// Output in this example is an sfn.StartExecutionOutput object.
	Output = sfn.StartExecutionOutput
)

func handler(_ context.Context, input interface{}) (Output, error) {

	// Create a Step Functions client named 'svc'
	sess := session.Must(session.NewSession())
	svc := sfn.New(sess)

	b, err := json.Marshal(input)
	if err != nil {
		return Output{}, err
	}
	inputString := string(b)

	stepFunctionInput := sfn.StartExecutionInput{
		Input:           aws.String(inputString),
		StateMachineArn: aws.String(os.Getenv("STATE_MACHINE_ARN")),
	}

	// Start the Step Function State Machine
	output, err := svc.StartExecution(&stepFunctionInput)
	if err != nil {
		return Output{}, err
	}

	return *output, nil
}

func main() {
	lambda.Start(handler)
}
