package main

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"net/http"
	"os"
	"strings"
	"trace-service/trace-service/bus"
	"trace-service/trace-service/domain"
)

var PodId = uuid.New().String()

func healthHandler(c *fiber.Ctx) error {
	logger.Info("Handle /health request")
	response := map[string]string{
		"status": "OK",
	}
	return c.Status(http.StatusOK).JSON(response)
}

func certificateHandler(c *fiber.Ctx) error {
	logger.Info("Handle /certificate request")
	return c.SendFile("localhost.crt")
}

func clientCertificateHandler(c *fiber.Ctx) error {
	logger.Info("Handle /client_certificate request")
	return c.SendFile("localhostclient.crt")
}

func privateKeyHandler(c *fiber.Ctx) error {
	logger.Info("Handle /private_key request")
	return c.SendFile("localhost.key")
}

func customResponseHeadersHandler(c *fiber.Ctx) error {
	logger.Info("Handle /custom-response-headers request")
	fastHttpCtx := c.Context()
	response := domain.TraceResponse{
		ServiceName: configloader.GetKoanf().MustString("microservice.name"),
		FamilyName:  configloader.GetOrDefaultString("service.name", ""),
		Version:     configloader.GetOrDefaultString("deployment.version", ""),
		RequestHost: string(fastHttpCtx.Host()),
		ServerHost:  os.Getenv("HOSTNAME"),
		RemoteAddr:  fastHttpCtx.RemoteAddr().String(),
		Path:        c.Path(),
		Method:      c.Method(),
		Headers:     ExtractRequestHeaders(c),
		PodId:       PodId,
	}

	var customHeaders map[string]string
	err := json.Unmarshal(c.Body(), &customHeaders)
	if err == nil {
		for key, value := range customHeaders {
			c.Set(key, value)
		}
	}

	logger.Infof("Responding with %+v %+v %+v %+v", response.ServiceName, response.ServerHost, response.Method, response.Path)
	return RespondWithJson(c, http.StatusOK, &response)
}

func busPublishHandler(c *fiber.Ctx) error {
	ctx := c.UserContext()
	topic := c.Params("topic")
	logger.InfoC(ctx, "Handle busPublishHandler request")
	fastHttpCtx := c.Context()
	response := domain.TraceResponse{
		ServiceName: configloader.GetKoanf().MustString("microservice.name"),
		FamilyName:  configloader.GetOrDefaultString("service.name", ""),
		Version:     configloader.GetOrDefaultString("deployment.version", ""),
		RequestHost: string(fastHttpCtx.Host()),
		ServerHost:  os.Getenv("HOSTNAME"),
		RemoteAddr:  fastHttpCtx.RemoteAddr().String(),
		Path:        c.Path(),
		Method:      c.Method(),
		Headers:     ExtractRequestHeaders(c),
		PodId:       PodId,
	}

	res, err := json.Marshal(&response)
	if err != nil {
		logger.ErrorC(ctx, "Failed to marshall grpc event: %v", err)
		return RespondWithJson(c, http.StatusInternalServerError, map[string]interface{}{"message": "Failed to publish gRPC event: " + err.Error()})
	}

	err = bus.Bus.Publish(topic, res)
	if err != nil {
		logger.ErrorC(ctx, "Failed to send grpc event: %v", err)
		return RespondWithJson(c, http.StatusInternalServerError, map[string]interface{}{"message": "Failed to publish gRPC event: " + err.Error()})
	}

	logger.InfoC(ctx, "Publishing event %+v", response)
	return RespondWithJson(c, http.StatusOK, map[string]interface{}{"message": "Event published successfully"})
}

func jsonTraceHandler(c *fiber.Ctx) error {
	logger.Info("Handle /trace request")
	fastHttpCtx := c.Context()
	path := string(c.Request().URI().Path())
	response := domain.TraceResponse{
		ServiceName: configloader.GetKoanf().MustString("microservice.name"),
		FamilyName:  configloader.GetOrDefaultString("service.name", ""),
		Version:     configloader.GetOrDefaultString("deployment.version", ""),
		PodId:       PodId,
		RequestHost: string(fastHttpCtx.Host()),
		ServerHost:  os.Getenv("HOSTNAME"),
		RemoteAddr:  fastHttpCtx.RemoteAddr().String(),
		Path:        path,
		Method:      c.Method(),
		Headers:     ExtractRequestHeaders(c),
	}

	logger.Infof("Responding with %+v %+v %+v %+v", response.ServiceName, response.ServerHost, response.Method, response.Path)
	return RespondWithJson(c, http.StatusOK, &response)
}

func ExtractRequestHeaders(c *fiber.Ctx) http.Header {
	result := make(map[string][]string)
	for key, value := range c.GetReqHeaders() {
		valueString := strings.Join(value, " ")
		result[key] = strings.Split(valueString, ",")
	}
	return result
}

func RespondWithJson(c *fiber.Ctx, code int, payload interface{}) error {
	c.Set("server", "this header must not be returned")
	c.Set("X-Forwarded-For", "this header must not be returned")
	c.Set("Some-Other-Header", "Header-value")
	return c.Status(code).JSON(payload)
}
