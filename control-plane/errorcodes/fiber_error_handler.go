package errorcodes

import (
	"github.com/gofiber/fiber/v3"
	errs "github.com/netcracker/qubership-core-lib-go-error-handling/v3/errors"
	fibererrors "github.com/netcracker/qubership-core-lib-go-fiber-server-utils/v3/errors"
)

func DefaultErrorHandlerWrapper(unknownErrorCode errs.ErrorCode) fiber.ErrorHandler {
	return func(ctx fiber.Ctx, err error) error {
		rootCauseErr := GetCpErrCodeErrorOrNil(err)
		if rootCauseErr != nil {
			return fibererrors.DefaultErrorHandler(unknownErrorCode)(ctx, rootCauseErr)
		}
		return fibererrors.DefaultErrorHandler(unknownErrorCode)(ctx, err)
	}
}
