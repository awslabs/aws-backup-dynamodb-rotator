// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
)

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
		SSMParameterName   string
	}

	// Input is an SNSEvent (a slice of SNSEventRecord)
	Input = events.SNSEvent

	// Output in this example is an sfn.StartExecutionOutput object.
	Output = sfn.StartExecutionOutput
)

func handler(_ context.Context, input Input) (Output, error) {
	// Get our source pattern regexp from the environment
	sourcePattern := os.Getenv("SOURCE_PATTERN")
	replacementPattern := os.Getenv("REPLACEMENT_PATTERN")
	stateMachineArn := os.Getenv("STATE_MACHINE_ARN")
	ssmParameterName := os.Getenv("SSM_PARAMETER_NAME")

	// Setup: Split the provided string into a BackupSnsMessage for parsing
	message, err := parseSnsInput(input)
	if err != nil {
		fmt.Printf("Unable to parse SNS input: %v\n", input)
		return Output{}, err
	}

	// If this is not a matching job, no need to kick off the state machine.
	match, err := isMatchingJob(message, sourcePattern)
	if err != nil {
		fmt.Println("Error matching message against pattern.")
		fmt.Printf("Message: (%v)\n", message)
		fmt.Printf("Pattern: %v\n", sourcePattern)
		return Output{}, err
	}

	if !match {
		fmt.Printf("Input does not require a restore: %v\n", input)
		return Output{}, nil
	}

	// We have a match; build our StepFunctionsInput
	bytes, err := json.Marshal(StepFunctionInput{
		BackupSnsMessage:   message,
		SourcePattern:      sourcePattern,
		ReplacementPattern: replacementPattern,
		SSMParameterName:   ssmParameterName,
	})
	if err != nil {
		fmt.Printf("Error marshalling message into JSON string: %v\n", message)
		return Output{}, err
	}

	stepFunctionInput := sfn.StartExecutionInput{
		Input:           aws.String(string(bytes)),
		StateMachineArn: aws.String(stateMachineArn),
	}

	// Create a Step Functions client named 'svc'
	sess := session.Must(session.NewSession())
	svc := sfn.New(sess)

	// Start the Step Function State Machine
	output, err := svc.StartExecution(&stepFunctionInput)
	if err != nil {
		fmt.Println("Error starting Step Function execution.")
		fmt.Printf("Step Function Input: %v\n", stepFunctionInput)
		fmt.Printf("State Machine ARN: %s\n", stateMachineArn)
		return Output{}, err
	}

	return *output, nil
}

func main() {
	lambda.Start(handler)
}

func isMatchingJob(message BackupSnsMessage, matchString string) (bool, error) {
	// Check to see if we have a match on the input
	// Match requires:
	//   1. this is a BACKUP_JOB_COMPLETE event
	//   2. the underlying resource is a DynamoDB table
	//   3. the underlying resource matches our SOURCE_PATTERN regexp pattern

	// 1. This must be a BACKUP_JOB_COMPLETE event. Currently this can be determined by
	//    matching on the string "An AWS Backup job was completed successfully."
	//    in the SNS message.
	const successfulBackupString = "An AWS Backup job was completed successfully."
	if !strings.Contains(message.StatusMessage, successfulBackupString) {
		fmt.Printf("This was not a BACKUP_JOB_COMPLETED notification: %v\n", message.StatusMessage)
		return false, nil
	}

	// 2. The backed up resource must be a DynamoDB table.
	const dynamoTableExpression = "(?i)^arn:aws:dynamodb:.*:.*:table/.*"
	isDynamoTable, err := regexp.MatchString(dynamoTableExpression, message.BackedUpResourceArn)
	if err != nil {
		fmt.Printf("Error matching backed up resource to DynamoDB regular expression: %s\n", message.BackedUpResourceArn)
		return false, err
	}
	if isDynamoTable == false {
		fmt.Printf("The backed up resource was not a DynamoDB table: %s\n", message.BackedUpResourceArn)
		return false, nil
	}

	// 3. The underlying resource matches our SOURCE_PATTERN regexp pattern.
	//    MatchString reports whether the string s contains any match of the regular
	//    expression pattern. More complicated queries need to use Compile and the
	//    full Regexp interface.
	return regexp.MatchString(matchString, message.BackedUpResourceArn)
}

func parseSnsInput(snsInput Input) (BackupSnsMessage, error) {

	record := snsInput.Records[0]
	snsMessage := record.SNS.Message

	b, err := json.MarshalIndent(snsInput.Records, "", "  ")
	if err != nil {
		return BackupSnsMessage{}, err
	}
	fmt.Printf("SNSEntity: %v\n", string(b))

	// Sample Message:
	//  "An AWS Backup job was completed successfully. Recovery point ARN: arn:aws:dynamodb:us-east-1:637093487455:table/MyDynamoDBTable/backup/01568804569000-d3306d76. Backed up Resource ARN : arn:aws:dynamodb:us-east-1:637093487455:table/MyDynamoDBTable. Backup Job Id : 5a772b5a-36d5-4a69-9b18-ed2f5213c659"
	//
	// So we need to extract:
	//   1. StatusMessage (everything before "Recovery point ARN: ")
	//   2. RecoveryPointArn (everything after "Recovery point ARN: " up until the period ".")
	//   3. BackedUpResourceArn (everything after "Backed Up Resource ARN : " up until the period ".")
	//   4. BackupJobId (everything after "Backup Job Id : ")

	// 1. Extract the StatusMessage
	firstSplitKey := "Recovery point ARN: "
	firstSplitString := strings.SplitAfter(snsMessage, firstSplitKey)
	if len(firstSplitString) == 0 {
		fmt.Printf("Could not parse StatusMessage from snsMessage: %v\n", snsMessage)
		return BackupSnsMessage{}, errors.New("parse failure: StatusMessage")
	}
	statusMessage := strings.SplitAfter(firstSplitString[0], ".")[0]

	// 2. Extract the RecoveryPointArn from the trailing string
	secondSplitKey := "."
	secondSplitString := strings.SplitAfterN(firstSplitString[1], secondSplitKey, 2)
	if len(secondSplitString) == 0 {
		fmt.Printf("Could not parse RecoveryPointArn for snsMessage: %v\n", snsMessage)
		return BackupSnsMessage{}, errors.New("parse failure: RecoveryPointArn")
	}
	recoveryPointArn := strings.Split(secondSplitString[0], ".")[0]

	// 3. Extract the BackedUpResourceArn from the trailing string
	thirdSplitKey := "Backed up Resource ARN : "
	thirdSplitString := strings.SplitAfter(secondSplitString[1], thirdSplitKey)
	if len(thirdSplitString) == 0 {
		fmt.Printf("Could not parse BackedUpResourceArn for snsMessage: %v\n", snsMessage)
		return BackupSnsMessage{}, errors.New("parse failure: BackedUpResourceArn")
	}
	tmpString := strings.SplitN(thirdSplitString[1], ".", 2)
	if len(tmpString) == 0 {
		fmt.Printf("Could not parse BackedUpResourceArn for snsMessage: %v\n", snsMessage)
		return BackupSnsMessage{}, errors.New("parse failure: BackedUpResourceArn")
	}
	backedUpResourceArn := tmpString[0]

	// 4. Extract the BackupJobId from the remaining string
	fourthSplitKey := "Backup Job Id : "
	fourthSplitString := strings.SplitAfter(tmpString[1], fourthSplitKey)
	if len(fourthSplitString) == 0 {
		fmt.Printf("Could not parse BackupJobID for snsMessage: %v\n", snsMessage)
		return BackupSnsMessage{}, errors.New("parse failure: BackupJobID")
	}
	backupJobID := fourthSplitString[1]

	// We also need to get the backup StartTime from the MessageAttributes so it can be
	// appended to the restored table.
	snapshotTimeAttribute := record.SNS.MessageAttributes["StartTime"].(map[string]interface{})
	snapshotTime := snapshotTimeAttribute["Value"].(string)
	// Backup StartTime is given in RFC3339 layout 2006-01-02T15:04:05Z07:00
	startTime, err := time.Parse(time.RFC3339, snapshotTime)
	if err != nil {
		return BackupSnsMessage{}, errors.New("parse failure: StartTime")
	}

	return BackupSnsMessage{
		StatusMessage:       statusMessage,
		RecoveryPointArn:    recoveryPointArn,
		BackedUpResourceArn: backedUpResourceArn,
		BackupJobID:         backupJobID,
		StartTime:           startTime,
	}, nil
}
