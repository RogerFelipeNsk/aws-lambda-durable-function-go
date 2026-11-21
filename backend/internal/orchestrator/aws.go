package orchestrator

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
)

type AWS struct {
	api          *lambda.Client
	functionName string
}

func NewAWS(api *lambda.Client, functionName string) *AWS {
	return &AWS{api: api, functionName: functionName}
}

func (a *AWS) StartCartSession(ctx context.Context, cartID string) (string, error) {
	payload, err := json.Marshal(domain.OrchestratorEvent{CartID: cartID})
	if err != nil {
		return "", err
	}
	out, err := a.api.Invoke(ctx, &lambda.InvokeInput{
		FunctionName:         aws.String(a.functionName),
		InvocationType:       types.InvocationTypeEvent,
		DurableExecutionName: aws.String(cartID),
		Payload:              payload,
	})
	if err != nil {
		return "", err
	}
	if out.DurableExecutionArn != nil {
		return *out.DurableExecutionArn, nil
	}
	return cartID, nil
}

func (a *AWS) SignalCheckout(ctx context.Context, callbackID string, signal domain.CheckoutSignal) error {
	raw, err := json.Marshal(signal)
	if err != nil {
		return err
	}
	_, err = a.api.SendDurableExecutionCallbackSuccess(ctx, &lambda.SendDurableExecutionCallbackSuccessInput{
		CallbackId: aws.String(callbackID),
		Result:     raw,
	})
	return err
}

func (a *AWS) SignalPixPaid(ctx context.Context, callbackID string) error {
	raw, err := json.Marshal(domain.PixSignal{Paid: true})
	if err != nil {
		return err
	}
	_, err = a.api.SendDurableExecutionCallbackSuccess(ctx, &lambda.SendDurableExecutionCallbackSuccessInput{
		CallbackId: aws.String(callbackID),
		Result:     raw,
	})
	return err
}

func (a *AWS) SignalFailure(ctx context.Context, callbackID, errorType, message string) error {
	_, err := a.api.SendDurableExecutionCallbackFailure(ctx, &lambda.SendDurableExecutionCallbackFailureInput{
		CallbackId: aws.String(callbackID),
		Error: &types.ErrorObject{
			ErrorType:    aws.String(errorType),
			ErrorMessage: aws.String(message),
		},
	})
	return err
}
