package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

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
	}

	//StepFunctionInput represents the input we pass to our state machine
	StepFunctionInput struct {
		BackupSnsMessage   BackupSnsMessage
		SourcePattern      string
		ReplacementPattern string
	}
	// SnsMessage is one message published by SNS
	SnsMessage struct {
		Message           string            `json:"Message"`
		MessageAttributes map[string]string `json:"MessageAttributes"`
		MessageID         string            `json:"MessageId"`
		Signature         string            `json:"Signature"`
		SignatureVersion  string            `json:"SignatureVersion"`
		SigningCertURL    string            `json:"SigningCertUrl"`
		Subject           string            `json:"Subject"`
		Timestamp         string            `json:"Timestamp"`
		TopicArn          string            `json:"TopicArn"`
		Type              string            `json:"Type"`
		UnsubscribeURL    string            `json:"UnsubscribeUrl"`
	}

	// SnsRecord is one SNS Message with its metadata
	SnsRecord struct {
		EventSource          string     `json:"EventSource"`
		EventSubscriptionArn string     `json:"EventSubscriptionArn"`
		EventVersion         string     `json:"EventVersion"`
		SnsMessage           SnsMessage `json:"Sns"`
	}

	// Input is a collection of SNS Records
	Input struct {
		Records []SnsRecord `json:"Records"`
	}

	// Output in this example is an sfn.StartExecutionOutput object.
	Output = sfn.StartExecutionOutput
)

func handler(_ context.Context, input Input) (Output, error) {
	// Get our source pattern regexp from the environment
	sourcePattern := os.Getenv("SOURCE_PATTERN")
	replacementPattern := os.Getenv("REPLACEMENT_PATTERN")
	stateMachineArn := os.Getenv("STATE_MACHINE_ARN")

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
	//   2. the underlying resource matches our SOURCE_PATTERN regexp pattern

	// 1. This must be a BACKUP_JOB_COMPLETE event. This is only be determined by
	//    matching on the string ""
	//    in the SNS message.
	const searchString = "An AWS Backup job was completed successfully."
	if !strings.Contains(message.StatusMessage, searchString) {
		fmt.Printf("This was not a BACKUP_COMPLETED job: %v\n", message.StatusMessage)
		return false, nil
	}

	// 2. The underlying resource matches our SOURCE_PATTERN regexp pattern.
	//    MatchString reports whether the string s contains any match of the regular
	//    expression pattern. More complicated queries need to use Compile and the
	//    full Regexp interface.
	fmt.Printf("Backed up resource ARN: %s\n", message.BackedUpResourceArn)
	return regexp.MatchString(matchString, message.BackedUpResourceArn)
}

func parseSnsInput(snsInput Input) (BackupSnsMessage, error) {

	snsMessage := snsInput.Records[0].SnsMessage.Message

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
	statusMessage := strings.SplitAfter(firstSplitString[0], ".")[0]

	// 2. Extract the RecoveryPointArn from the trailing string
	secondSplitKey := "."
	secondSplitString := strings.SplitAfterN(firstSplitString[1], secondSplitKey, 2)
	recoveryPointArn := strings.Split(secondSplitString[0], ".")[0]

	// 3. Extract the BackedUpResourceArn from the trailing string
	thirdSplitKey := "Backed up Resource ARN : "
	thirdSplitString := strings.SplitAfter(secondSplitString[1], thirdSplitKey)
	tmpString := strings.SplitN(thirdSplitString[1], ".", 2)
	backedUpResourceArn := tmpString[0]

	// 4. Extract the BackupJobId from the remaining string
	fourthSplitKey := "Backup Job Id : "
	fourthSplitString := strings.SplitAfter(tmpString[1], fourthSplitKey)
	backupJobID := fourthSplitString[1]

	return BackupSnsMessage{
		StatusMessage:       statusMessage,
		RecoveryPointArn:    recoveryPointArn,
		BackedUpResourceArn: backedUpResourceArn,
		BackupJobID:         backupJobID,
	}, nil
}
