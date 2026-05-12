package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"elevator-go/internal/interface/http/oapi"
)

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Elevator Simulator API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.addEventListener('load', () => {
      window.ui = SwaggerUIBundle({
        url: '/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
      });
    });
  </script>
</body>
</html>
`

// 生成済みの埋め込み spec を流用する。docs/openapi.yaml を別途複製しないので一次ソースは一つに保たれる。
func SpecHandler() (http.HandlerFunc, error) {
	swagger, err := oapi.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("load embedded openapi spec: %w", err)
	}
	body, err := json.Marshal(swagger)
	if err != nil {
		return nil, fmt.Errorf("marshal openapi spec: %w", err)
	}
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}, nil
}

func SwaggerUIHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerUIHTML))
	}
}
