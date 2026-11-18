package errorcodes

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	errs "github.com/netcracker/qubership-core-lib-go-error-handling/v3/errors"
	"github.com/netcracker/qubership-core-lib-go-error-handling/v3/tmf"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

var testError1 = errs.ErrorCode{Code: "CORE-MESH-CP-0001", Title: "Test title 1"}
var testError2 = errs.ErrorCode{Code: "CORE-MESH-CP-0002", Title: "Test title 2"}
var testError3 = errs.ErrorCode{Code: "CORE-MESH-CP-0003", Title: "Test title 3"}

func TestMultiCauseErrorWithTwoDifferentErrors(t *testing.T) {
	causes := []CpErrCodeError{
		*NewRestErrorWithMeta(testError1, "testError1", http.StatusConflict, "test1"),
		*NewRestErrorWithMeta(testError2, "testError2", http.StatusBadRequest, "test2"),
	}
	multiCauseError := NewMultiCauseError(testCpError, causes)

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := multiCauseError.Handle(ctx)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusInternalServerError, ctx.Response().StatusCode())
	tmfResponse := readTmfResponse(t, ctx)
	assert.Equal(t, testCpError.Code, tmfResponse.Code)

	errors := *tmfResponse.Errors
	assert.Equal(t, len(causes), len(errors))
	assert.True(t, containError(causes, errors[0]))
	assert.True(t, containError(causes, errors[1]))

	assert.Equal(t, strconv.Itoa(http.StatusInternalServerError), *tmfResponse.Status)
	assert.Equal(t, "testError1; testError2", tmfResponse.Message)
}

func TestMultiCauseErrorWithTwoSimilarErrors(t *testing.T) {
	causes := []CpErrCodeError{
		*NewRestErrorWithMeta(testError2, "testError2", http.StatusBadRequest, "test2"),
		*NewRestErrorWithMeta(testError3, "testError3", http.StatusBadRequest, "test3"),
	}
	multiCauseError := NewMultiCauseError(testCpError, causes)

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := multiCauseError.Handle(ctx)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusBadRequest, ctx.Response().StatusCode())
	tmfResponse := readTmfResponse(t, ctx)
	assert.Equal(t, testCpError.Code, tmfResponse.Code)

	errors := *tmfResponse.Errors
	assert.Equal(t, len(causes), len(errors))
	assert.True(t, containError(causes, errors[0]))
	assert.True(t, containError(causes, errors[1]))

	assert.Equal(t, strconv.Itoa(http.StatusBadRequest), *tmfResponse.Status)
	assert.Equal(t, "testError2; testError3", tmfResponse.Message)
}

func TestMultiCauseErrorWithThreeErrors(t *testing.T) {
	causes := []CpErrCodeError{
		*NewRestErrorWithMeta(testError1, "testError1", http.StatusConflict, "test1"),
		*NewRestErrorWithMeta(testError2, "testError2", http.StatusBadRequest, "test2"),
		*NewRestErrorWithMeta(testError3, "testError3", http.StatusBadRequest, "test3"),
	}
	multiCauseError := NewMultiCauseError(testCpError, causes)

	app := fiber.New()
	ctx := app.AcquireCtx(&fasthttp.RequestCtx{})

	err := multiCauseError.Handle(ctx)
	assert.Nil(t, err)
	assert.Equal(t, testCpError.httpCode, ctx.Response().StatusCode())
	tmfResponse := readTmfResponse(t, ctx)
	assert.Equal(t, testCpError.Code, tmfResponse.Code)

	errors := *tmfResponse.Errors
	assert.Equal(t, len(causes), len(errors))
	assert.True(t, containError(causes, errors[0]))
	assert.True(t, containError(causes, errors[1]))
	assert.True(t, containError(causes, errors[2]))

	assert.Equal(t, strconv.Itoa(http.StatusInternalServerError), *tmfResponse.Status)
	assert.Equal(t, "testError1; testError2; See 1 more errors in errors[*].message", tmfResponse.Message)
}

func containError(expectedErrors []CpErrCodeError, actualErr tmf.Error) bool {
	for _, expErr := range expectedErrors {
		if strconv.Itoa(expErr.httpCode) == *actualErr.Status {
			return actualErr.Meta != nil
		}
	}

	return false
}

func readTmfResponse(t *testing.T, ctx fiber.Ctx) tmf.Response {
	tmfResponse := tmf.Response{}
	err := json.Unmarshal(ctx.Response().Body(), &tmfResponse)
	assert.Nil(t, err)

	return tmfResponse
}
