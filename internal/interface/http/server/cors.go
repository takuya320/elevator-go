package server

import "net/http"

// CORS_ALLOWED_ORIGINS に列挙した Origin にだけ ACAO を返す。
// "*" の単一指定なら無条件全許可。credentials は使わない前提（EventSource も
// withCredentials=false で動かす）なので ACAC は付けない。
// 空スライスなら no-op を返す（同一オリジン専用デプロイで余計なヘッダを出さないため）。
func CORS(allowed []string) func(http.Handler) http.Handler {
	if len(allowed) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	wildcard := len(allowed) == 1 && allowed[0] == "*"
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if wildcard {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := set[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
				}
			}
			// preflight: Access-Control-Request-Method がある OPTIONS だけを CORS preflight と扱う。
			// 他の OPTIONS は通常のルーティングに渡す。
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				if h := r.Header.Get("Access-Control-Request-Headers"); h != "" {
					w.Header().Set("Access-Control-Allow-Headers", h)
				}
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
