// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// These types will change based on whatever your function needs to
// consume and return. In this case we are passed a dynamodb.CreateTableInput
// object and return the dynamodb.CreateTableOutput from our call to
// dynamodb.CreateTable.

type (
	// BackupSnsMessage represents the broken out SNS Message Body sent in
	// an AWS Backup SNS message.
	BackupSnsMessage struct {
		StatusMessage       string
		RecoveryPointArn    string
		BackedUpResourceArn string
		BackupJobID         string
		StartTime           time.Time
	}

	//StepFunctionInput represents the input we pass to our state machine
	StepFunctionInput struct {
		BackupSnsMessage   BackupSnsMessage
		SourcePattern      string
		ReplacementPattern string
	}

	// Input is a single StepFunctionInput record
	Input = StepFunctionInput

	// Output in this example is a dynamodb.CreateTableOutput object.
	Output = dynamodb.RestoreTableFromBackupOutput
)

func handler(_ context.Context, input Input) (Output, error) {
	// Create a DynamoDB client named 'svc'
	sess := session.Must(session.NewSession())
	svc := dynamodb.New(sess)

	// Parse input records to extract the required fields to restore
	// a DynamoDB table
	dynamoDBInput, err := parseInput(input)
	if err != nil {
		return Output{}, err
	}

	// Create the DynamoDB Table
	output, err := svc.RestoreTableFromBackup(&dynamoDBInput)
	if err != nil {
		return Output{}, err
	}

	return *output, nil
}

func main() {
	lambda.Start(handler)
}

func parseInput(input Input) (dynamodb.RestoreTableFromBackupInput, error) {
	// First, split the table name out from the backed up resource ARN
	parts := strings.SplitAfter(input.BackupSnsMessage.BackedUpResourceArn, "table/")
	if len(parts) < 2 {
		fmt.Println("Could not split DynamoDB table name from backed up resource ARN.")
		return dynamodb.RestoreTableFromBackupInput{}, errors.New("Bad input ARN")
	}
	tableName := parts[1]

	// Next, apply our replacement expression to get a new base tablename
	r, err := regexp.Compile(input.SourcePattern)
	if err != nil {
		fmt.Printf("Could not compile source expression: %s\n", input.SourcePattern)
		return dynamodb.RestoreTableFromBackupInput{}, err
	}

	replacement := r.ReplaceAllString(tableName, input.ReplacementPattern)
	var str strings.Builder
	str.WriteString(replacement)

	// Append a restore date-time stamp in the format "-YYYYMMDD-HH-mm-ss"
	t := input.BackupSnsMessage.StartTime
	str.WriteString(t.Format("-20060102-15-04-05"))
	targetTable := str.String()

	// Restore the backed up table
	return dynamodb.RestoreTableFromBackupInput{
		BackupArn:       aws.String(input.BackupSnsMessage.RecoveryPointArn),
		TargetTableName: aws.String(targetTable),
	}, nil
}
