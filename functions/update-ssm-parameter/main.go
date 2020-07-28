// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// These types will change based on whatever your function needs to
// consume and return. In this case we are passed a dynamodb.CreateTableInput
// object and return the dynamodb.CreateTableOutput from our call to
// dynamodb.CreateTable.

type (
	// Input is our entire Step Function input object, but we're only concerned
	// with the DynamoDB table name and the SSM parameter name.
	Input struct {
		DescribeTableOutput dynamodb.DescribeTableOutput
		SSMParameterName    string
	}

	// Output in this example is a dynamodb.CreateTableOutput object.
	Output = ssm.PutParameterOutput
)

func handler(_ context.Context, input Input) (Output, error) {
	// Create an SSM client named 'svc'
	sess := session.Must(session.NewSession())
	svc := ssm.New(sess)

	// Create our input record
	ssmInput := ssm.PutParameterInput{
		Name:      aws.String(input.SSMParameterName),
		Overwrite: aws.Bool(true),
		Type:      aws.String(ssm.ParameterTypeString),
		Value:     input.DescribeTableOutput.Table.TableArn,
	}

	// Update the SSM Parameter
	output, err := svc.PutParameter(&ssmInput)
	if err != nil {
		return Output{}, err
	}

	return *output, nil
}

func main() {
	lambda.Start(handler)
}
