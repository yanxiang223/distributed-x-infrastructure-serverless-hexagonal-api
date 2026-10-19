package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"huaweicloud.com/go-runtime/events/apig"
	runtimecontext "huaweicloud.com/go-runtime/go-api/context"

	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/domain"
)

// responseSerializationReserve 在函数执行超时预算中预留给响应序列化的时间。
const responseSerializationReserve = 200 * time.Millisecond

// APIGHandler 是 API 网关触发的入站适配器（六边形架构的 driving adapter）。
// 它把网关事件翻译为领域调用，再把领域结果翻译为网关响应；
// 业务规则全部位于领域层，本层不含任何业务判断。
type APIGHandler struct {
	products *domain.Products
}

func NewAPIGHandler(d *domain.Products) *APIGHandler {
	return &APIGHandler{
		products: d,
	}
}

// AllHandler 处理 GET /，返回一页商品。
func (l *APIGHandler) AllHandler(ctx context.Context, event apig.APIGTriggerEvent) (apig.APIGTriggerResponse, error) {
	next := event.QueryStringParameters["next"]

	productRange, err := l.products.AllProducts(ctx, &next)
	if err != nil {
		return errResponse(http.StatusInternalServerError, err.Error()), nil
	}

	return response(http.StatusOK, productRange), nil
}

// GetHandler 处理 GET /{id}，返回单个商品。
func (l *APIGHandler) GetHandler(ctx context.Context, event apig.APIGTriggerEvent) (apig.APIGTriggerResponse, error) {
	id, ok := pathParam(event, "id")
	if !ok {
		return errResponse(http.StatusBadRequest, "missing 'id' parameter in path"), nil
	}

	product, err := l.products.GetProduct(ctx, id)

	if err != nil {
		return errResponse(http.StatusInternalServerError, err.Error()), nil
	}
	if product == nil {
		return errResponse(http.StatusNotFound, "product not found"), nil
	} else {
		return response(http.StatusOK, product), nil
	}
}

// PutHandler 处理 PUT /{id}，写入商品并在成功后发布领域事件。
func (l *APIGHandler) PutHandler(ctx context.Context, event apig.APIGTriggerEvent) (apig.APIGTriggerResponse, error) {
	id, ok := pathParam(event, "id")
	if !ok {
		return errResponse(http.StatusBadRequest, "missing 'id' parameter in path"), nil
	}

	body, err := requestBody(event)
	if err != nil {
		return errResponse(http.StatusBadRequest, err.Error()), nil
	}

	if strings.TrimSpace(body) == "" {
		return errResponse(http.StatusBadRequest, "empty request body"), nil
	}

	product, err := l.products.PutProduct(ctx, id, []byte(body))
	if err != nil {
		if errors.Is(err, domain.ErrJsonUnmarshal) || errors.Is(err, domain.ErrProductIdMismatch) {
			return errResponse(http.StatusBadRequest, err.Error()), nil
		} else {
			return errResponse(http.StatusInternalServerError, err.Error()), nil
		}
	}

	return response(http.StatusCreated, product), nil
}

// DeleteHandler 处理 DELETE /{id}，删除商品并在成功后发布领域事件。
func (l *APIGHandler) DeleteHandler(ctx context.Context, event apig.APIGTriggerEvent) (apig.APIGTriggerResponse, error) {
	id, ok := pathParam(event, "id")
	if !ok {
		return errResponse(http.StatusBadRequest, "missing 'id' parameter in path"), nil
	}

	err := l.products.DeleteProduct(ctx, id)
	if err != nil {
		return errResponse(http.StatusInternalServerError, err.Error()), nil
	}

	return response(http.StatusOK, nil), nil
}

// RuntimeHandler 把业务入口适配为函数工作流 Go 运行时的入口签名。
// 运行时以 (payload []byte, ctx RuntimeContext) 反射调用入口函数，
// 本适配器负责事件反序列化与执行超时预算的桥接。
func RuntimeHandler(handle func(context.Context, apig.APIGTriggerEvent) (apig.APIGTriggerResponse, error)) func([]byte, runtimecontext.RuntimeContext) (interface{}, error) {
	return func(payload []byte, runtimeCtx runtimecontext.RuntimeContext) (interface{}, error) {
		var event apig.APIGTriggerEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return nil, fmt.Errorf("cannot unmarshal API gateway trigger event: %w", err)
		}

		execCtx, cancel := executionContext(runtimeCtx)
		defer cancel()

		return handle(execCtx, event)
	}
}

// executionContext 依据运行时给出的剩余时间预算构造带截止时间的执行上下文，
// 使领域调用能在函数被强制中断前主动收敛。
func executionContext(runtimeCtx runtimecontext.RuntimeContext) (context.Context, context.CancelFunc) {
	remaining := runtimeCtx.GetRemainingTimeInMilliSeconds()
	if remaining <= 0 {
		return context.WithCancel(context.Background())
	}

	budget := time.Duration(remaining)*time.Millisecond - responseSerializationReserve
	if budget <= 0 {
		budget = time.Duration(remaining) * time.Millisecond
	}

	return context.WithTimeout(context.Background(), budget)
}

// pathParam 读取路径参数。网关按 API 定义映射路径参数；
// 当映射未配置时回退到从请求路径末段解析，保证 `/{id}` 的语义稳定。
func pathParam(event apig.APIGTriggerEvent, name string) (string, bool) {
	if value, ok := event.PathParameters[name]; ok && strings.TrimSpace(value) != "" {
		return value, true
	}

	segments := strings.Split(strings.Trim(event.Path, "/"), "/")
	last := segments[len(segments)-1]
	if strings.TrimSpace(last) == "" {
		return "", false
	}

	return last, true
}

// requestBody 还原请求体。网关以 isBase64Encoded 标记说明 body 是否需要解码。
func requestBody(event apig.APIGTriggerEvent) (string, error) {
	if !event.IsBase64Encoded {
		return event.Body, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(event.Body)
	if err != nil {
		return "", fmt.Errorf("invalid base64 request body: %w", err)
	}

	return string(decoded), nil
}

func response(code int, object interface{}) apig.APIGTriggerResponse {
	marshalled, err := json.Marshal(object)
	if err != nil {
		return errResponse(http.StatusInternalServerError, err.Error())
	}

	return apig.APIGTriggerResponse{
		StatusCode: code,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body:            string(marshalled),
		IsBase64Encoded: false,
	}
}

func errResponse(status int, body string) apig.APIGTriggerResponse {
	message := map[string]string{
		"message": body,
	}

	messageBytes, _ := json.Marshal(&message)

	return apig.APIGTriggerResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(messageBytes),
	}
}
