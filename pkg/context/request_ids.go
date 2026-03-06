package context

import (
	"context"
	"crypto/rand"
	"fmt"
)

type requestIDCtxKey struct{}
type operationIDCtxKey struct{}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey{}, requestID)
}

func RequestID(ctx context.Context) (string, bool) {
	requestID, ok := ctx.Value(requestIDCtxKey{}).(string)
	return requestID, ok
}

func WithOperationID(ctx context.Context, operationID string) context.Context {
	return context.WithValue(ctx, operationIDCtxKey{}, operationID)
}

func OperationID(ctx context.Context) (string, bool) {
	operationID, ok := ctx.Value(operationIDCtxKey{}).(string)
	return operationID, ok
}

func GenerateRequestID() (string, error) {
	return generateID("req")
}

func GenerateOperationID() (string, error) {
	return generateID("op")
}

func generateID(prefix string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}
	return fmt.Sprintf("%s_%x", prefix, buf), nil
}
