//go:build unit
// +build unit

package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"

	"huaweicloud.com/go-runtime/events/apig"

	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/domain"
	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/store"
	"github.com/huaweicloud-samples/distributed-x-infrastructure-serverless-hexagonal-api/types"
)

func newTestHandler() *APIGHandler {
	return NewAPIGHandler(domain.NewProductsDomain(store.NewMemoryStore()))
}

func apiGatewayEvent(method, path, body string, pathParameters map[string]string, base64Encoded bool) apig.APIGTriggerEvent {
	return apig.APIGTriggerEvent{
		HttpMethod:            method,
		Path:                  path,
		Body:                  body,
		PathParameters:        pathParameters,
		QueryStringParameters: map[string]string{},
		IsBase64Encoded:       base64Encoded,
	}
}

func decodeBody(t *testing.T, resp apig.APIGTriggerResponse, target interface{}) {
	t.Helper()

	if err := json.Unmarshal([]byte(resp.Body), target); err != nil {
		t.Fatalf("cannot decode response body %q: %v", resp.Body, err)
	}
}

func TestPutThenGetProduct(t *testing.T) {
	ctx := context.Background()
	handler := newTestHandler()

	body := `{"id":"iXR","name":"iPhone XML","price":0.123}`
	putResp, err := handler.PutHandler(ctx, apiGatewayEvent(http.MethodPut, "/iXR", body, map[string]string{"id": "iXR"}, false))
	if err != nil {
		t.Fatalf("PutHandler returned an error: %v", err)
	}

	if putResp.StatusCode != http.StatusCreated {
		t.Fatalf("Expecting 201, got %d (%s)", putResp.StatusCode, putResp.Body)
	}

	getResp, err := handler.GetHandler(ctx, apiGatewayEvent(http.MethodGet, "/iXR", "", map[string]string{"id": "iXR"}, false))
	if err != nil {
		t.Fatalf("GetHandler returned an error: %v", err)
	}

	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("Expecting 200, got %d (%s)", getResp.StatusCode, getResp.Body)
	}

	product := types.Product{}
	decodeBody(t, getResp, &product)

	if product.Id != "iXR" || product.Name != "iPhone XML" || product.Price != 0.123 {
		t.Errorf("Got unexpected product: %+v", product)
	}
}

func TestGetHandlerReturnsNotFound(t *testing.T) {
	handler := newTestHandler()

	resp, err := handler.GetHandler(context.Background(), apiGatewayEvent(http.MethodGet, "/missing", "", map[string]string{"id": "missing"}, false))
	if err != nil {
		t.Fatalf("GetHandler returned an error: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expecting 404, got %d", resp.StatusCode)
	}
}

func TestGetHandlerFallsBackToPathSegment(t *testing.T) {
	ctx := context.Background()
	handler := newTestHandler()

	body := `{"id":"iXR","name":"iPhone XML","price":0.123}`
	handler.PutHandler(ctx, apiGatewayEvent(http.MethodPut, "/iXR", body, map[string]string{"id": "iXR"}, false))

	// 未配置路径参数映射时，仍能从请求路径末段解析出 id
	resp, err := handler.GetHandler(ctx, apiGatewayEvent(http.MethodGet, "/iXR", "", nil, false))
	if err != nil {
		t.Fatalf("GetHandler returned an error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expecting 200, got %d (%s)", resp.StatusCode, resp.Body)
	}
}

func TestGetHandlerWithoutId(t *testing.T) {
	handler := newTestHandler()

	resp, err := handler.GetHandler(context.Background(), apiGatewayEvent(http.MethodGet, "/", "", nil, false))
	if err != nil {
		t.Fatalf("GetHandler returned an error: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expecting 400, got %d", resp.StatusCode)
	}
}

func TestPutHandlerRejectsEmptyBody(t *testing.T) {
	handler := newTestHandler()

	resp, err := handler.PutHandler(context.Background(), apiGatewayEvent(http.MethodPut, "/iXR", "   ", map[string]string{"id": "iXR"}, false))
	if err != nil {
		t.Fatalf("PutHandler returned an error: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expecting 400, got %d", resp.StatusCode)
	}

	message := map[string]string{}
	decodeBody(t, resp, &message)

	if message["message"] != "empty request body" {
		t.Errorf("Got unexpected message: %s", message["message"])
	}
}

func TestPutHandlerRejectsInvalidBody(t *testing.T) {
	handler := newTestHandler()

	resp, err := handler.PutHandler(context.Background(), apiGatewayEvent(http.MethodPut, "/iXR", "invalid body", map[string]string{"id": "iXR"}, false))
	if err != nil {
		t.Fatalf("PutHandler returned an error: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expecting 400, got %d", resp.StatusCode)
	}
}

func TestPutHandlerRejectsIdMismatch(t *testing.T) {
	handler := newTestHandler()

	body := `{"id":"iXR","name":"iPhone XML","price":0.123}`
	resp, err := handler.PutHandler(context.Background(), apiGatewayEvent(http.MethodPut, "/another-id", body, map[string]string{"id": "another-id"}, false))
	if err != nil {
		t.Fatalf("PutHandler returned an error: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expecting 400, got %d", resp.StatusCode)
	}

	message := map[string]string{}
	decodeBody(t, resp, &message)

	if message["message"] != domain.ErrProductIdMismatch.Error() {
		t.Errorf("Got unexpected message: %s", message["message"])
	}
}

func TestPutHandlerDecodesBase64Body(t *testing.T) {
	handler := newTestHandler()

	payload := `{"id":"iXR","name":"iPhone XML","price":0.123}`
	encoded := base64.StdEncoding.EncodeToString([]byte(payload))

	resp, err := handler.PutHandler(context.Background(), apiGatewayEvent(http.MethodPut, "/iXR", encoded, map[string]string{"id": "iXR"}, true))
	if err != nil {
		t.Fatalf("PutHandler returned an error: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expecting 201, got %d (%s)", resp.StatusCode, resp.Body)
	}
}

func TestPutHandlerRejectsInvalidBase64Body(t *testing.T) {
	handler := newTestHandler()

	resp, err := handler.PutHandler(context.Background(), apiGatewayEvent(http.MethodPut, "/iXR", "not-base64!!", map[string]string{"id": "iXR"}, true))
	if err != nil {
		t.Fatalf("PutHandler returned an error: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expecting 400, got %d", resp.StatusCode)
	}
}

func TestDeleteHandlerRemovesProduct(t *testing.T) {
	ctx := context.Background()
	handler := newTestHandler()

	body := `{"id":"iXR","name":"iPhone XML","price":0.123}`
	handler.PutHandler(ctx, apiGatewayEvent(http.MethodPut, "/iXR", body, map[string]string{"id": "iXR"}, false))

	deleteResp, err := handler.DeleteHandler(ctx, apiGatewayEvent(http.MethodDelete, "/iXR", "", map[string]string{"id": "iXR"}, false))
	if err != nil {
		t.Fatalf("DeleteHandler returned an error: %v", err)
	}

	if deleteResp.StatusCode != http.StatusOK {
		t.Fatalf("Expecting 200, got %d", deleteResp.StatusCode)
	}

	getResp, _ := handler.GetHandler(ctx, apiGatewayEvent(http.MethodGet, "/iXR", "", map[string]string{"id": "iXR"}, false))
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("Expecting the product to be removed, got %d", getResp.StatusCode)
	}
}

func TestDeleteHandlerWithoutId(t *testing.T) {
	handler := newTestHandler()

	resp, err := handler.DeleteHandler(context.Background(), apiGatewayEvent(http.MethodDelete, "/", "", nil, false))
	if err != nil {
		t.Fatalf("DeleteHandler returned an error: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expecting 400, got %d", resp.StatusCode)
	}
}

func TestAllHandlerReturnsProducts(t *testing.T) {
	ctx := context.Background()
	handler := newTestHandler()

	body := `{"id":"iXR","name":"iPhone XML","price":0.123}`
	handler.PutHandler(ctx, apiGatewayEvent(http.MethodPut, "/iXR", body, map[string]string{"id": "iXR"}, false))

	resp, err := handler.AllHandler(ctx, apiGatewayEvent(http.MethodGet, "/", "", nil, false))
	if err != nil {
		t.Fatalf("AllHandler returned an error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expecting 200, got %d", resp.StatusCode)
	}

	productRange := types.ProductRange{}
	decodeBody(t, resp, &productRange)

	if len(productRange.Products) != 1 {
		t.Errorf("Expecting 1 product, got %d", len(productRange.Products))
	}
}

func TestResponsesAreNotBase64Encoded(t *testing.T) {
	handler := newTestHandler()

	resp, err := handler.AllHandler(context.Background(), apiGatewayEvent(http.MethodGet, "/", "", nil, false))
	if err != nil {
		t.Fatalf("AllHandler returned an error: %v", err)
	}

	if resp.IsBase64Encoded {
		t.Error("Expecting plain response body")
	}

	if resp.Headers["Content-Type"] != "application/json" {
		t.Errorf("Got unexpected content type: %s", resp.Headers["Content-Type"])
	}
}
