package restutils

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/url"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/utils/v2"
)

func RespondWithError(c fiber.Ctx, code int, msg string) error {
	return RespondWithJson(c, code, map[string]string{"error": msg})
}

func RespondWithJson(c fiber.Ctx, code int, payload interface{}) error {
	return c.Status(code).JSON(payload)
}

func RespondWithZip(c fiber.Ctx, code int, payload interface{}, filename string, archivename string) error {
	ctx := c.Context()
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	defer func(zipWriter *zip.Writer) {
		errClose := zipWriter.Close()
		if errClose != nil {
			log.Error(ctx, errClose, "failed to finish the zip compress process")
		}
	}(zipWriter)

	f, err := zipWriter.Create(filename)
	if err != nil {
		return err
	}

	data, err := c.App().Config().JSONEncoder(payload)
	if err != nil {
		return err
	}

	_, err = f.Write(data)
	if err != nil {
		return err
	}

	err = zipWriter.Close()
	if err != nil {
		return err
	}

	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", "attachment; filename="+archivename)
	return c.Send(buf.Bytes())
}

func ResponseOk(c fiber.Ctx, payload interface{}) error {
	return RespondWithJson(c, http.StatusOK, payload)
}

func ResponseNoContent(c fiber.Ctx, payload interface{}) error {
	return RespondWithJson(c, http.StatusNoContent, payload)
}

func GetFiberParam(fiberCtx fiber.Ctx, paramName string) string {
	paramValue := fiberCtx.Params(paramName)
	unescapedStr, err := url.QueryUnescape(paramValue)
	if err != nil {
		return utils.CopyString(paramValue)
	}
	return utils.CopyString(unescapedStr)
}
