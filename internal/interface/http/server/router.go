package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"elevator-go/internal/interface/http/oapi"
)

// handler / sse は外部から注入する（main 側に UseCase 組み立てを集約するため）。
// corsOrigins が空なら CORS 無効。frontend を別オリジンに置く構成のときだけ main から渡す。
func NewRouter(handler oapi.ServerInterface, sse http.Handler, corsOrigins []string) (http.Handler, error) {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// chi v5.3+: RealIP は IP spoofing 脆弱性で deprecated。クライアント IP を
	// 読む箇所が無いので外す（必要なら ClientIPFrom* をデプロイ構成に合わせて選ぶ）。
	r.Use(RequestLogger(slog.Default()))
	r.Use(middleware.Recoverer)
	r.Use(CORS(corsOrigins))

	specHandler, err := SpecHandler()
	if err != nil {
		return nil, err
	}
	r.Get("/openapi.json", specHandler)
	r.Get("/docs", SwaggerUIHandler())
	r.Get("/docs/", SwaggerUIHandler())

	r.Get("/events", sse.ServeHTTP)

	oapi.HandlerFromMux(handler, r)

	staticHandler, err := StaticHandler()
	if err != nil {
		return nil, err
	}
	// API・SSE・Swagger UI で拾われなかったパスは React build を返す。
	r.NotFound(staticHandler.ServeHTTP)

	return r, nil
}
