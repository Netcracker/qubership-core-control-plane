package errorcodes

import (
	"github.com/gofiber/fiber/v2"
	errs "github.com/netcracker/qubership-core-lib-go-error-handling/v3/errors"
	"github.com/netcracker/qubership-core-lib-go-error-handling/v3/tmf"
)

type cpRestErrorCode struct {
	errs.ErrorCode
	httpCode        int
	printStackTrace bool
}

type CpErrCodeError struct {
	*errs.ErrCodeError
	httpCode        int
	meta            *map[string]interface{}
	printStackTrace bool
}

func (e CpErrCodeError) Handle(ctx *fiber.Ctx) error {
	if e.printStackTrace {
		logger.ErrorC(ctx.UserContext(), "%s", errs.ToLogFormat(e))
	} else {
		logger.ErrorC(ctx.UserContext(), "%s", errs.ToLogFormatWithoutStackTrace(e))
	}
	response := tmf.ErrToResponse(e, e.GetHttpCode())
	return ctx.Status(e.GetHttpCode()).JSON(response)
}

func (e CpErrCodeError) GetHttpCode() int {
	return e.httpCode
}

func (e cpRestErrorCode) GetHttpCode() int {
	return e.httpCode
}
