package middleware

import (
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/instill-ai/model-backend/pkg/constant"
	"github.com/instill-ai/model-backend/pkg/service"
)

type fn func(service.Service, http.ResponseWriter, *http.Request, map[string]string)

func AppendCustomHeaderMiddleware(s service.Service, next fn) runtime.HandlerFunc {
	return runtime.HandlerFunc(func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		r.Header.Add(constant.HeaderOwnerIDKey, constant.DefaultOwnerID)
		next(s, w, r, pathParams)
	})
}
