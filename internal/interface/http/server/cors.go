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
			originAllowed := false
			if origin != "" {
				if wildcard {
					w.Header().Set("Access-Control-Allow-Origin", "*")
					originAllowed = true
				} else if _, ok := set[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Add("Vary", "Origin")
					originAllowed = true
				}
			}
			// preflight 短絡は「許可した Origin の preflight」だけに限定する。
			// 拒否 Origin / Origin 無しの OPTIONS は通常ルートに流し、必要なら 404/405 を返させる
			// （ACAO 無しの 204 は仕様バグに見えるし、他用途の OPTIONS を吸ってしまうため）。
			isPreflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
			if isPreflight && originAllowed {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				// Request-Headers を echo するので、それごとに 204 が変わる → Vary に積む。
				// CDN/逆プロキシが Origin A の preflight を Origin B に再利用しないため。
				w.Header().Add("Vary", "Access-Control-Request-Headers")
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
