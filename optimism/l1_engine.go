package optimism

import (
	"context"
	"fmt"
	"github.com/ethereum-optimism/optimism/op-node/eth"
	"github.com/ethereum/go-ethereum/rpc"
	"time"
)

type L1EngineClient struct {
	client *rpc.Client
}

// ForkchoiceUpdate updates the forkchoice on the execution client. If attributes is not nil, the engine client will also begin building a block
// based on attributes after the new head block and return the payload ID.
//
// The RPC may return three types of errors:
// 1. Processing error: ForkchoiceUpdatedResult.PayloadStatusV1.ValidationError or other non-success PayloadStatusV1,
// 2. `error` as eth.InputError: the forkchoice state or attributes are not valid.
// 3. Other types of `error`: temporary RPC errors, like timeouts.
func (s *L1EngineClient) ForkchoiceUpdate(ctx context.Context, fc *eth.ForkchoiceState, attributes *eth.PayloadAttributes) (*eth.ForkchoiceUpdatedResult, error) {
	if attributes.NoTxPool || len(attributes.Transactions) != 0 {
		panic("cannot use L2 engine API attributes with L1 Node")
	}
	fcCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	var result eth.ForkchoiceUpdatedResult
	err := s.client.CallContext(fcCtx, &result, "engine_forkchoiceUpdatedV1", fc, attributes)
	if err == nil {
		return &result, nil
	} else {
		if rpcErr, ok := err.(rpc.Error); ok {
			code := eth.ErrorCode(rpcErr.ErrorCode())
			switch code {
			case eth.InvalidForkchoiceState, eth.InvalidPayloadAttributes:
				return nil, eth.InputError{
					Inner: err,
					Code:  code,
				}
			default:
				return nil, fmt.Errorf("unrecognized rpc error: %w", err)
			}
		}
		return nil, err
	}
}

// NewPayload executes a full block on the execution engine.
// This returns a PayloadStatusV1 which encodes any validation/processing error,
// and this type of error is kept separate from the returned `error` used for RPC errors, like timeouts.
func (s *L1EngineClient) NewPayload(ctx context.Context, payload *eth.ExecutionPayload) (*eth.PayloadStatusV1, error) {
	execCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	var result eth.PayloadStatusV1
	err := s.client.CallContext(execCtx, &result, "engine_newPayloadV1", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to execute payload: %w", err)
	}
	return &result, nil
}

// GetPayload gets the execution payload associated with the PayloadId.
// There may be two types of error:
// 1. `error` as eth.InputError: the payload ID may be unknown
// 2. Other types of `error`: temporary RPC errors, like timeouts.
func (s *L1EngineClient) GetPayload(ctx context.Context, payloadId eth.PayloadID) (*eth.ExecutionPayload, error) {
	var result eth.ExecutionPayload
	err := s.client.CallContext(ctx, &result, "engine_getPayloadV1", payloadId)
	if err != nil {
		if rpcErr, ok := err.(rpc.Error); ok {
			code := eth.ErrorCode(rpcErr.ErrorCode())
			switch code {
			case eth.UnknownPayload:
				return nil, eth.InputError{
					Inner: err,
					Code:  code,
				}
			default:
				return nil, fmt.Errorf("unrecognized rpc error: %w", err)
			}
		}
		return nil, err
	}
	return &result, nil
}
