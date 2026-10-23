package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"verify-logistics-signup/internal/logistics"
)

func main() {
	client, err := logistics.NewInfraiClient()
	if err != nil {
		log.Fatal(err)
	}
	service := logistics.Service{Users: client, Emails: client}
	http.HandleFunc("/signups", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		var signup logistics.Signup
		if err := json.NewDecoder(r.Body).Decode(&signup); err != nil {
			http.Error(w, "invalid signup JSON", http.StatusBadRequest)
			return
		}
		result, err := service.StartSignup(r.Context(), signup)
		if err != nil {
			var apiErr *logistics.InfraiError
			if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
				http.Error(w, apiErr.Message, apiErr.Status)
				return
			}
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	})
	log.Println("logistics signup listener on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
