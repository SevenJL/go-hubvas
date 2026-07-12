package handler

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func cropContext(values url.Values) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c
}

func TestParseCropAllowsAbsentCrop(t *testing.T) {
	crop, err := parseCrop(cropContext(url.Values{}))
	if err != nil || crop != nil {
		t.Fatalf("expected absent crop, got %#v, %v", crop, err)
	}
}

func TestParseCropRejectsPartialOrMalformedCrop(t *testing.T) {
	if _, err := parseCrop(cropContext(url.Values{"crop_x": {"0"}})); err == nil {
		t.Fatal("expected partial crop to be rejected")
	}
	values := url.Values{"crop_x": {"nope"}, "crop_y": {"0"}, "crop_width": {"1"}, "crop_height": {"1"}}
	if _, err := parseCrop(cropContext(values)); err == nil {
		t.Fatal("expected malformed crop to be rejected")
	}
}

func TestParseCropReadsCompleteCrop(t *testing.T) {
	values := url.Values{"crop_x": {"0.1"}, "crop_y": {"0.2"}, "crop_width": {"0.5"}, "crop_height": {"0.5"}}
	crop, err := parseCrop(cropContext(values))
	if err != nil {
		t.Fatal(err)
	}
	if crop == nil || crop.X != .1 || crop.Y != .2 || crop.Width != .5 || crop.Height != .5 {
		t.Fatalf("unexpected crop: %#v", crop)
	}
}
