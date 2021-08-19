package handlerx

// func RegisterHandlers1(r *chi.Mux, srv http.Handler) {
// 	r.Handle("/rest/{id}", srv)
// 	r.Handle("/rest/hosts/{id}", srv)
// 	r.Handle("/rest/hosts", srv)

// 	r.Method("PUT", "/rest/host/{id}", srv)

// 	r.Route("/api/v1", func(r chi.Router) {
// 		r.Head("/{id}", func(w http.ResponseWriter, r *http.Request) {
// 			w.Header().Set("X-User", "-")
// 			w.Write([]byte("user"))
// 		})
// 		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
// 			id := chi.URLParam(r, "id")
// 			rctx := chi.RouteContext(r.Context())
// 			x := rctx.URLParams
// 			y := strings.Join(x.Keys, ",") + "=" + strings.Join(x.Values, ",")
// 			w.Header().Set("X-User", id)
// 			z := fmt.Sprintf("%#v", r.URL.Query())
// 			w.Write([]byte("user:" + id + "; " + y + "; " + z))
// 		})
// 		r.MethodFunc("GET", "/test", func(w http.ResponseWriter, r *http.Request) {
// 			w.Header().Set("X-User", "-")
// 			w.Write([]byte("test"))
// 		})
// 	})
// }
