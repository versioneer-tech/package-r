package http

import (
	"io/fs"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/versioneer-tech/package-r/settings"
	"github.com/versioneer-tech/package-r/storage"
)

func NewHandler(
	imgSvc ImgService,
	fileCache FileCache,
	store *storage.Storage,
	server *settings.Server,
	assetsFs fs.FS,
) (http.Handler, error) {
	server.Clean()

	r := mux.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", `default-src 'self'; style-src 'unsafe-inline';`)
			next.ServeHTTP(w, r)
		})
	})
	index, static := getStaticHandlers(store, server, assetsFs)

	// Keep object paths unchanged. Cleaning a URL can change a valid object key.
	r = r.SkipClean(true)

	wrap := func(fn handleFunc, prefix string) http.Handler {
		return handle(fn, prefix, store, server)
	}

	r.HandleFunc("/health", healthHandler)
	r.PathPrefix("/static").Handler(static)
	r.NotFoundHandler = index

	api := r.PathPrefix("/api").Subrouter()
	api.NotFoundHandler = http.NotFoundHandler()

	tokenExpirationTime := server.GetTokenExpirationTime(DefaultTokenExpirationTime)
	api.Handle("/login", wrap(loginHandler(tokenExpirationTime), "")).Methods("POST")
	api.Handle("/renew", wrap(renewHandler(tokenExpirationTime), "")).Methods("POST")

	api.PathPrefix("/resources").Handler(wrap(resourceGetHandler, "/api/resources")).Methods("GET")
	api.PathPrefix("/resources").Handler(wrap(resourceDeleteHandler(fileCache), "/api/resources")).Methods("DELETE")
	api.PathPrefix("/resources").Handler(wrap(resourcePostHandler(fileCache), "/api/resources")).Methods("POST")
	api.PathPrefix("/resources").Handler(wrap(resourcePutHandler, "/api/resources")).Methods("PUT")
	api.PathPrefix("/resources").Handler(wrap(resourcePatchHandler(fileCache), "/api/resources")).Methods("PATCH")

	api.PathPrefix("/tus").Handler(wrap(tusPostHandler(), "/api/tus")).Methods("POST")
	api.PathPrefix("/tus").Handler(wrap(tusHeadHandler(), "/api/tus")).Methods("HEAD")
	api.PathPrefix("/tus").Handler(wrap(tusPatchHandler(), "/api/tus")).Methods("PATCH")
	api.PathPrefix("/tus").Handler(wrap(resourceDeleteHandler(fileCache), "/api/tus")).Methods("DELETE")

	api.Handle("/shares", wrap(configuredShareGetsHandler, "/api/shares")).Methods("GET")
	api.PathPrefix("/shares").Handler(http.NotFoundHandler())

	api.PathPrefix("/raw").Handler(wrap(rawHandler, "/api/raw")).Methods("GET")
	api.PathPrefix("/preview/{size}/{path:.*}").
		Handler(wrap(previewHandler(imgSvc, fileCache, server.EnableThumbnails, server.ResizePreview), "/api/preview")).Methods("GET")
	api.PathPrefix("/command").Handler(wrap(commandsHandler, "/api/command")).Methods("GET")

	public := api.PathPrefix("/public").Subrouter()
	public.PathPrefix("/share").Handler(wrap(publicShareHandler, "/api/public/share/")).Methods("GET", "HEAD")
	public.PathPrefix("/catalog").Handler(wrap(catalogHandler, "/api/public/catalog/")).Methods("GET", "HEAD")

	return stripPrefix(server.BaseURL, r), nil
}
