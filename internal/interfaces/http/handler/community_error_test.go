package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hubvas/internal/domain/shared"
	"github.com/hubvas/internal/interfaces/http/response"
)

func TestSocialErrorUsesHTTPStatusAndHidesSentinelText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	socialError(ctx, shared.NewDomainError(shared.ErrNotFound, "parent comment not found"))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
	var body response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Message != "parent comment not found" {
		t.Fatalf("unexpected public message %q", body.Message)
	}
	if strings.Contains(body.Message, "domain:") {
		t.Fatalf("domain sentinel leaked to client: %q", body.Message)
	}
}
