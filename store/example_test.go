package store_test

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/epistemer/cfgx"
	"github.com/epistemer/cfgx/store"
)

// A small admin endpoint over a command's settings. The generated command
// type's Store method returns st; pass it to the handler like any other
// dependency.
func Example() {
	var st *store.Store // from s.Store(), in a command that has the store

	mux := http.NewServeMux()
	// Every setting, with its value, where it came from, and the stored
	// change that set it, if one did.
	mux.HandleFunc("GET /settings", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(st.List())
	})
	// Set one mutable key from the request body, as canonical text such as
	// "250" or "5m". The whole configuration is validated first; an error
	// means nothing changed.
	mux.HandleFunc("PUT /settings/{key}", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<16))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		err = st.Update(r.Context(), map[string]string{r.PathValue("key"): string(body)},
			cfgx.By(r.Header.Get("X-User")), cfgx.Reason(r.Header.Get("X-Reason")))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// The last 20 changes to a key, newest first.
	mux.HandleFunc("GET /settings/{key}/history", func(w http.ResponseWriter, r *http.Request) {
		h, err := st.History(r.PathValue("key"), 20)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest) // usually an unknown key
			return
		}
		json.NewEncoder(w).Encode(h)
	})
	_ = mux
}
