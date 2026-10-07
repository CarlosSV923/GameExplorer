package httpserver_test

import (
	"net/http"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpserver"
)

func TestNewKeepsTransferTimeoutsOpen(t *testing.T) {
	t.Parallel()

	srv := httpserver.New(":0", http.NewServeMux())

	if srv.ReadTimeout != 0 || srv.WriteTimeout != 0 {
		t.Fatalf("read/write timeouts must be 0 for large transfers, got %v/%v", srv.ReadTimeout, srv.WriteTimeout)
	}
	if srv.ReadHeaderTimeout == 0 {
		t.Fatal("ReadHeaderTimeout must be set")
	}
}
