package errorcodes

import (
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"
	errs "github.com/netcracker/qubership-core-lib-go-error-handling/v3/errors"
	"github.com/netcracker/qubership-core-lib-go-error-handling/v3/tmf"
)

type cpMultiCauseError struct {
	*errs.MultiCauseError
	causes   []CpErrCodeError
	httpCode int
}

func (e cpMultiCauseError) GetCauses() []errs.ErrCodeErr {
	var errorCodeErrs []errs.ErrCodeErr
	for _, cause := range e.causes {
		errorCodeErrs = append(errorCodeErrs, cause)
	}
	return errorCodeErrs
}

func (e cpMultiCauseError) Handle(ctx fiber.Ctx) error {
	response := tmf.ErrToResponse(e, e.httpCode)

	var multiErrors []tmf.Error
	for i, err := range *response.Errors {
		status := strconv.Itoa(e.causes[i].httpCode)
		err.Status = &status
		err.Meta = e.causes[i].meta
		multiErrors = append(multiErrors, err)
	}
	response.Errors = &multiErrors
	return ctx.Status(e.httpCode).JSON(response)
}

func NewMultiCauseError(errCode cpRestErrorCode, causes []CpErrCodeError) *cpMultiCauseError {
	detail := causes[0].Detail
	if len(causes) == 2 {
		detail = causes[0].Detail + "; " + causes[1].Detail
	}
	if len(causes) > 2 {
		detail = causes[0].Detail + "; " + causes[1].Detail + "; See " + strconv.Itoa(len(causes)-2) + " more errors in errors[*].message"
	}

	httpCode := causes[0].httpCode
	for _, cause := range causes {
		if cause.httpCode != httpCode {
			httpCode = http.StatusInternalServerError
			break
		}
	}

	return &cpMultiCauseError{
		MultiCauseError: errs.NewMultiCauseError(errCode.ErrorCode, detail, nil),
		causes:          causes,
		httpCode:        httpCode,
	}
}
