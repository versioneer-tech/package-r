package http

import (
	"errors"
	"fmt"
	stdHTTP "net/http"
	"testing"
)

type testResponseError struct {
	status int
}

func (e testResponseError) Error() string {
	return "object storage request failed"
}

func (e testResponseError) HTTPStatusCode() int {
	return e.status
}

func TestErrToStatusMapsObjectStorageAuthorizationErrors(t *testing.T) {
	for _, status := range []int{stdHTTP.StatusUnauthorized, stdHTTP.StatusForbidden} {
		err := fmt.Errorf("read object: %w", testResponseError{status: status})
		if got := errToStatus(err); got != stdHTTP.StatusForbidden {
			t.Fatalf("errToStatus() = %d, want %d", got, stdHTTP.StatusForbidden)
		}
	}
}

func TestErrToStatusDoesNotMapOtherObjectStorageErrorsToForbidden(t *testing.T) {
	err := fmt.Errorf("read object: %w", testResponseError{status: stdHTTP.StatusBadGateway})
	if got := errToStatus(err); got != stdHTTP.StatusInternalServerError {
		t.Fatalf("errToStatus() = %d, want %d", got, stdHTTP.StatusInternalServerError)
	}
}

func TestErrToStatusHandlesUnknownError(t *testing.T) {
	if got := errToStatus(fmt.Errorf("read object: %w", errors.New("other"))); got != stdHTTP.StatusInternalServerError {
		t.Fatalf("errToStatus() = %d, want %d", got, stdHTTP.StatusInternalServerError)
	}
}
