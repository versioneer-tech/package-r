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

	// NOTE: This fixes the issue where it would redirect if people did not put a
	// trailing slash in the end. I hate this decision since this allows some awful
	// URLs https://www.gorillatoolkit.org/pkg/mux#Router.SkipClean
	r = r.SkipClean(true)

	monkey := func(fn handleFunc, prefix string) http.Handler {
		return handle(fn, prefix, store, server)
	}

	r.HandleFunc("/health", healthHandler)
	r.PathPrefix("/static").Handler(static)
	r.NotFoundHandler = index

	api := r.PathPrefix("/api").Subrouter()
	api.NotFoundHandler = http.NotFoundHandler()

	tokenExpirationTime := server.GetTokenExpirationTime(DefaultTokenExpirationTime)
	api.Handle("/login", monkey(loginHandler(tokenExpirationTime), ""))
	api.Handle("/renew", monkey(renewHandler(tokenExpirationTime), ""))

	api.PathPrefix("/resources").Handler(monkey(resourceGetHandler, "/api/resources")).Methods("GET")
	api.PathPrefix("/resources").Handler(monkey(resourceDeleteHandler(fileCache), "/api/resources")).Methods("DELETE")
	api.PathPrefix("/resources").Handler(monkey(resourcePostHandler(fileCache), "/api/resources")).Methods("POST")
	api.PathPrefix("/resources").Handler(monkey(resourcePutHandler, "/api/resources")).Methods("PUT")
	api.PathPrefix("/resources").Handler(monkey(resourcePatchHandler(fileCache), "/api/resources")).Methods("PATCH")

	api.PathPrefix("/tus").Handler(monkey(tusPostHandler(), "/api/tus")).Methods("POST")
	api.PathPrefix("/tus").Handler(monkey(tusHeadHandler(), "/api/tus")).Methods("HEAD")
	api.PathPrefix("/tus").Handler(monkey(tusPatchHandler(), "/api/tus")).Methods("PATCH")
	api.PathPrefix("/tus").Handler(monkey(resourceDeleteHandler(fileCache), "/api/tus")).Methods("DELETE")

	api.PathPrefix("/shares").Handler(http.NotFoundHandler())
	api.PathPrefix("/share").Handler(monkey(configuredShareGetsHandler, "/api/share")).Methods("GET")
	api.PathPrefix("/share").Handler(http.NotFoundHandler())

	api.PathPrefix("/raw").Handler(monkey(rawHandler, "/api/raw")).Methods("GET")
	api.PathPrefix("/preview/{size}/{path:.*}").
		Handler(monkey(previewHandler(imgSvc, fileCache, server.EnableThumbnails, server.ResizePreview), "/api/preview")).Methods("GET")
	api.PathPrefix("/command").Handler(monkey(commandsHandler, "/api/command")).Methods("GET")
	api.PathPrefix("/subtitle").Handler(monkey(subtitleHandler, "/api/subtitle")).Methods("GET")

	public := api.PathPrefix("/public").Subrouter()
	public.PathPrefix("/dl").Handler(monkey(publicDlHandler, "/api/public/dl/")).Methods("GET")
	public.PathPrefix("/share").Handler(monkey(publicShareHandler, "/api/public/share/")).Methods("GET", "HEAD")
	public.PathPrefix("/catalog").Handler(monkey(catalogHandler, "/api/public/catalog/")).Methods("GET", "HEAD")

	return stripPrefix(server.BaseURL, r), nil
}
