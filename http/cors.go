package http

import (
	"net/http"
	"strings"
)

const (
	corsAllowedMethods = "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS"
	corsAllowedHeaders = "Authorization, Content-Type, Range, Tus-Resumable, Upload-Concat, Upload-Defer-Length, Upload-Length, Upload-Metadata, Upload-Offset, X-Auth, X-SHARE-PASSWORD"
	corsExposedHeaders = "Accept-Ranges, Content-Disposition, Content-Length, Content-Range, ETag, Location, Tus-Resumable, Upload-Length, Upload-Offset, X-Renew-Token"
)

func corsMiddleware(configuredOrigins string, next http.Handler) http.Handler {
	allowedOrigins := parseAllowedOrigins(configuredOrigins)
	if len(allowedOrigins) == 0 {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigin, allowed := corsAllowedOrigin(allowedOrigins, origin)
		if !allowed {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Expose-Headers", corsExposedHeaders)
		if allowedOrigin != "*" {
			w.Header().Add("Vary", "Origin")
		}

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseAllowedOrigins(configuredOrigins string) map[string]struct{} {
	origins := make(map[string]struct{})
	for _, origin := range strings.Split(configuredOrigins, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return origins
}

func corsAllowedOrigin(allowedOrigins map[string]struct{}, origin string) (string, bool) {
	if origin == "" {
		return "", false
	}
	if _, allowed := allowedOrigins["*"]; allowed {
		return "*", true
	}
	_, allowed := allowedOrigins[origin]
	return origin, allowed
}
