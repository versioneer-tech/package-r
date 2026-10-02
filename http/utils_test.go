package http

import (
	"errors"
	"fmt"
	stdHTTP "net/http"
	"testing"
)

type xyzResponseError struct {
	status int
}

func (e xyzResponseError) Error() string {
	return "object storage request failed"
}

func (e xyzResponseError) HTTPStatusCode() int {
	return e.status
}

func TestErrToStatusMapsObjectStorageAuthorizationErrors(t *testing.T) {
	for _, status := range []int{stdHTTP.StatusUnauthorized, stdHTTP.StatusForbidden} {
		err := fmt.Errorf("read object: %w", xyzResponseError{status: status})
		if got := errToStatus(err); got != stdHTTP.StatusForbidden {
			t.Fatalf("errToStatus() = %d, want %d", got, stdHTTP.StatusForbidden)
		}
	}
}

func TestErrToStatusDoesNotMapOtherObjectStorageErrorsToForbidden(t *testing.T) {
	err := fmt.Errorf("read object: %w", xyzResponseError{status: stdHTTP.StatusBadGateway})
	if got := errToStatus(err); got != stdHTTP.StatusInternalServerError {
		t.Fatalf("errToStatus() = %d, want %d", got, stdHTTP.StatusInternalServerError)
	}
}

func TestErrToStatusHandlesWrappedPermissionError(t *testing.T) {
	if got := errToStatus(fmt.Errorf("read object: %w", errors.New("other"))); got != stdHTTP.StatusInternalServerError {
		t.Fatalf("errToStatus() = %d, want %d", got, stdHTTP.StatusInternalServerError)
	}
}
